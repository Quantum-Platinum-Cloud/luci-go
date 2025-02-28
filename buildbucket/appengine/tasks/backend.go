// Copyright 2022 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tasks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/api/googleapi"

	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	cipdpb "go.chromium.org/luci/cipd/api/cipd/v1"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/retry/transient"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/caching/layered"
	"go.chromium.org/luci/server/tq"

	"go.chromium.org/luci/buildbucket/appengine/internal/buildtoken"
	"go.chromium.org/luci/buildbucket/appengine/internal/config"
	"go.chromium.org/luci/buildbucket/appengine/internal/metrics"
	"go.chromium.org/luci/buildbucket/appengine/model"
	pb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/buildbucket/protoutil"
)

const (
	// bbagentReservedGracePeriod is the time reserved by bbagent in order to have
	// time to have a couple retry rounds for UpdateBuild RPCs
	// TODO(crbug.com/1328646): may need to adjust the grace_period based on
	// UpdateBuild's new performance in Buildbucket Go.
	bbagentReservedGracePeriod = 180

	// runTaskGiveUpTimeout indicates how long to retry
	// the CreateBackendTask before giving up with INFRA_FAILURE.
	runTaskGiveUpTimeout = 10 * 60 * time.Second

	// buildStartGiveUpTimeout indicates how long the build has
	// to start before givin up with INFRA_FAILURE.
	buildStartGiveUpTimeout = 60 * 60 * time.Second

	cipdCacheTTL = 10 * time.Minute
)

type cipdPackageDetails struct {
	Size int64  `json:"size,omitempty"`
	Hash string `json:"hash,omitempty"`
}

type cipdPackageDetailsMap map[string]*cipdPackageDetails

var cipdDescribeBootstrapBundleCache = layered.RegisterCache(layered.Parameters[cipdPackageDetailsMap]{
	ProcessCacheCapacity: 1000,
	GlobalNamespace:      "cipd-describeBootstrapBundle-v1",
	Marshal: func(item cipdPackageDetailsMap) ([]byte, error) {
		return json.Marshal(item)
	},
	Unmarshal: func(blob []byte) (cipdPackageDetailsMap, error) {
		res := cipdPackageDetailsMap{}
		err := json.Unmarshal(blob, &res)
		return res, err
	},
})

type MockTaskBackendClientKey struct{}

type MockCipdClientKey struct{}

// BackendClient is the client to communicate with TaskBackend.
// It wraps a pb.TaskBackendClient.

type BackendClient struct {
	client TaskBackendClient
}

type TaskBackendClient interface {
	RunTask(ctx context.Context, taskReq *pb.RunTaskRequest, opts ...grpc.CallOption) (*pb.RunTaskResponse, error)
}

func createRawPrpcClient(ctx context.Context, host, project string) (client *prpc.Client, err error) {
	t, err := auth.GetRPCTransport(ctx, auth.AsProject, auth.WithProject(project))
	if err != nil {
		return nil, err
	}
	client = &prpc.Client{
		C:       &http.Client{Transport: t},
		Host:    host,
		Options: prpc.DefaultOptions(),
	}
	return
}

func newRawTaskBackendClient(ctx context.Context, host string, project string) (TaskBackendClient, error) {
	if mockClient, ok := ctx.Value(MockTaskBackendClientKey{}).(TaskBackendClient); ok {
		return mockClient, nil
	}
	prpcClient, err := createRawPrpcClient(ctx, host, project)
	if err != nil {
		return nil, err
	}
	return pb.NewTaskBackendPRPCClient(prpcClient), nil
}

// NewBackendClient creates a client to communicate with Buildbucket.
func NewBackendClient(ctx context.Context, bld *pb.Build, infra *pb.BuildInfra) (*BackendClient, error) {
	hostnname, err := computeHostnameFromTarget(ctx, infra.Backend.Task.Id.Target)
	if err != nil {
		return nil, err
	}
	client, err := newRawTaskBackendClient(ctx, hostnname, bld.Builder.Project)
	if err != nil {
		return nil, err
	}
	return &BackendClient{
		client: client,
	}, nil
}

// RunTask returns for the requested task.
func (c *BackendClient) RunTask(ctx context.Context, taskReq *pb.RunTaskRequest, opts ...grpc.CallOption) (*pb.RunTaskResponse, error) {
	return c.client.RunTask(ctx, taskReq)
}

func NewCipdClient(ctx context.Context, host string, project string) (client *prpc.Client, err error) {
	if mockClient, ok := ctx.Value(MockCipdClientKey{}).(*prpc.Client); ok {
		return mockClient, nil
	}
	client, err = createRawPrpcClient(ctx, host, project)
	return
}

func computeHostnameFromTarget(ctx context.Context, target string) (hostname string, err error) {
	globalCfg, err := config.GetSettingsCfg(ctx)
	if err != nil {
		return "", errors.Annotate(err, "could not get global settings config").Err()
	}
	for _, config := range globalCfg.Backends {
		if config.Target == target {
			return config.Hostname, nil
		}
	}
	return "", errors.Reason("could not find target in global config settings").Err()
}

// computeTaskCaches computes the task caches.
func computeTaskCaches(infra *model.BuildInfra) []*pb.CacheEntry {
	caches := make([]*pb.CacheEntry, len(infra.Proto.Backend.GetCaches()))
	for i, c := range infra.Proto.Backend.GetCaches() {
		caches[i] = &pb.CacheEntry{
			EnvVar:           c.GetEnvVar(),
			Name:             c.GetName(),
			Path:             c.GetPath(),
			WaitForWarmCache: c.GetWaitForWarmCache(),
		}
	}
	return caches
}

func computeAgentArgs(build *pb.Build, infra *pb.BuildInfra) (args []string) {
	args = []string{}
	// build-id arg
	args = append(args, "-build-id")
	args = append(args, strconv.FormatInt(build.GetId(), 10))
	// host arg
	args = append(args, "-host")
	args = append(args, infra.Buildbucket.GetHostname())
	// cache-base arg
	args = append(args, "-cache-base")
	args = append(args, infra.Bbagent.GetCacheDir())
	return
}

func computeBackendNewTaskReq(ctx context.Context, build *model.Build, infra *model.BuildInfra) (*pb.RunTaskRequest, error) {
	// Create task token and secrets.
	registerTaskToken, err := buildtoken.GenerateToken(ctx, build.ID, pb.TokenBody_REGISTER_TASK)
	if err != nil {
		return nil, err
	}
	startBuildToken, err := buildtoken.GenerateToken(ctx, build.ID, pb.TokenBody_START_BUILD)
	if err != nil {
		return nil, err
	}

	secrets := &pb.BuildSecrets{
		StartBuildToken:               startBuildToken,
		ResultdbInvocationUpdateToken: build.ResultDBUpdateToken,
	}
	backend := infra.Proto.GetBackend()
	if backend == nil {
		return nil, errors.New("infra.Proto.Backend isn't set")
	}
	caches := computeTaskCaches(infra)
	if err != nil {
		return nil, errors.Annotate(err, "RunTaskRequest.Caches could not be created").Err()
	}
	gracePeriod := &durationpb.Duration{
		Seconds: build.Proto.GetGracePeriod().GetSeconds() + bbagentReservedGracePeriod,
	}

	startDeadline := &timestamppb.Timestamp{
		Seconds: build.Proto.GetCreateTime().GetSeconds() + int64(buildStartGiveUpTimeout.Seconds()),
	}

	taskReq := &pb.RunTaskRequest{
		BuildbucketHost:          infra.Proto.Buildbucket.Hostname,
		RegisterBackendTaskToken: registerTaskToken,
		Secrets:                  secrets,
		Target:                   backend.Task.Id.Target,
		RequestId:                uuid.New().String(),
		BuildId:                  strconv.FormatInt(build.Proto.Id, 10),
		Realm:                    build.Realm(),
		BackendConfig:            backend.Config,
		ExecutionTimeout:         build.Proto.GetExecutionTimeout(),
		GracePeriod:              gracePeriod,
		Caches:                   caches,
		AgentArgs:                computeAgentArgs(build.Proto, infra.Proto),
		Dimensions:               infra.Proto.Backend.GetTaskDimensions(),
		StartDeadline:            startDeadline,
		Experiments:              build.Proto.Input.GetExperiments(),
	}

	project := build.Proto.Builder.Project
	taskReq.Agent = &pb.RunTaskRequest_AgentExecutable{}
	taskReq.Agent.Source, err = extractCipdDetails(ctx, project, infra.Proto)
	if err != nil {
		return nil, err
	}
	return taskReq, nil
}

func createCipdDescribeBootstrapBundleRequest(infra *pb.BuildInfra) *cipdpb.DescribeBootstrapBundleRequest {
	prefix := infra.Buildbucket.Agent.Source.GetCipd().GetPackage()
	prefix = strings.TrimSuffix(prefix, "/${platform}")
	return &cipdpb.DescribeBootstrapBundleRequest{
		Prefix:  prefix,
		Version: infra.Buildbucket.Agent.Source.GetCipd().GetVersion(),
	}
}

func computeCipdURL(source *pb.BuildInfra_Buildbucket_Agent_Source, pkg string, details *cipdPackageDetails) (url string) {
	server := source.GetCipd().GetServer()
	version := source.GetCipd().GetVersion()
	return server + "/bootstrap/" + pkg + "/+/" + version
}

// extractCipdDetails returns a map that maps package (Prefix + variant for each variant)
// to a cipdPackageDetails object, which is just the hash and size.
//
// A Cipd client is created and calls DescribeBootstrapBundle to retrieve the data.
func extractCipdDetails(ctx context.Context, project string, infra *pb.BuildInfra) (details map[string]*pb.RunTaskRequest_AgentExecutable_AgentSource, err error) {
	cipdServer := infra.Buildbucket.Agent.Source.GetCipd().GetServer()
	cipdClient, err := NewCipdClient(ctx, cipdServer, project)
	if err != nil {
		return nil, err
	}
	req := createCipdDescribeBootstrapBundleRequest(infra)
	bytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	cachePrefix := base64.StdEncoding.EncodeToString(bytes)
	cipdDetails, err := cipdDescribeBootstrapBundleCache.GetOrCreate(ctx, cachePrefix, func() (cipdPackageDetailsMap, time.Duration, error) {
		out := &cipdpb.DescribeBootstrapBundleResponse{}
		err := cipdClient.Call(ctx, "cipd.Repository", "DescribeBootstrapBundle", req, out)
		if err != nil {
			return nil, 0, err
		}
		resp := make(cipdPackageDetailsMap, len(out.Files))
		for _, file := range out.Files {
			resp[file.Package] = &cipdPackageDetails{
				Hash: file.Instance.HexDigest,
				Size: file.Size,
			}
		}
		return resp, cipdCacheTTL, nil
	})
	if err != nil {
		return nil, errors.Annotate(err, "cache error for cipd request").Err()
	}
	details = map[string]*pb.RunTaskRequest_AgentExecutable_AgentSource{}
	for k, v := range cipdDetails {
		val := &pb.RunTaskRequest_AgentExecutable_AgentSource{
			Sha256:    v.Hash,
			SizeBytes: v.Size,
			Url:       computeCipdURL(infra.Buildbucket.Agent.Source, k, v),
		}
		details[k] = val
	}
	return
}

// failBuild fails the given build with INFRA_FAILURE status.
func failBuild(ctx context.Context, buildID int64, msg string) error {
	bld := &model.Build{
		ID: buildID,
	}

	var changedToEnded bool
	err := datastore.RunInTransaction(ctx, func(ctx context.Context) error {
		switch err := datastore.Get(ctx, bld); {
		case err == datastore.ErrNoSuchEntity:
			logging.Warningf(ctx, "build %d not found: %s", buildID, err)
			return nil
		case err != nil:
			return errors.Annotate(err, "failed to fetch build: %d", bld.ID).Err()
		}

		changedToEnded = !protoutil.IsEnded(bld.Proto.Status)
		if !changedToEnded {
			// Build already ended, no more change to it.
			return nil
		}

		protoutil.SetStatus(clock.Now(ctx), bld.Proto, pb.Status_INFRA_FAILURE)
		bld.Proto.SummaryMarkdown = msg

		toSave := []any{bld}
		bs := &model.BuildStatus{Build: datastore.KeyForObj(ctx, bld)}
		switch err := datastore.Get(ctx, bs); {
		case err == datastore.ErrNoSuchEntity:
			// This is allowed during BuildStatus rollout.
			// TODO(crbug.com/1430324): also check ErrNoSuchEntity.
		case err != nil:
			return errors.Annotate(err, "failed to fetch build status: %d", bld.ID).Err()
		default:
			bs.Status = pb.Status_INFRA_FAILURE
			toSave = append(toSave, bs)
		}

		if err := sendOnBuildCompletion(ctx, bld); err != nil {
			return err
		}

		return datastore.Put(ctx, toSave)
	}, nil)
	if err != nil {
		return transient.Tag.Apply(errors.Annotate(err, "failed to terminate build: %d", buildID).Err())
	}
	if changedToEnded {
		metrics.BuildCompleted(ctx, bld)
	}
	return nil
}

// CreateBackendTask creates a backend task for the build.
func CreateBackendTask(ctx context.Context, buildID int64) error {
	bld := &model.Build{ID: buildID}
	infra := &model.BuildInfra{Build: datastore.KeyForObj(ctx, bld)}
	switch err := datastore.Get(ctx, bld, infra); {
	case errors.Contains(err, datastore.ErrNoSuchEntity):
		return tq.Fatal.Apply(errors.Annotate(err, "build %d or buildInfra not found", buildID).Err())
	case err != nil:
		return transient.Tag.Apply(errors.Annotate(err, "failed to fetch build %d or buildInfra", buildID).Err())
	}

	// Create a backend task client
	backend, err := NewBackendClient(ctx, bld.Proto, infra.Proto)
	if err != nil {
		return tq.Fatal.Apply(errors.Annotate(err, "failed to connect to backend service").Err())
	}

	taskReq, err := computeBackendNewTaskReq(ctx, bld, infra)
	if err != nil {
		return tq.Fatal.Apply(err)
	}

	// Create a backend task via RunTask
	_, err = backend.RunTask(ctx, taskReq)
	if err != nil {
		// Give up if HTTP 500s are happening continuously. Otherwise re-throw the
		// error so Cloud Tasks retries the task.
		if apiErr, _ := err.(*googleapi.Error); apiErr == nil || apiErr.Code >= 500 {
			if clock.Now(ctx).Sub(bld.CreateTime) < runTaskGiveUpTimeout {
				return transient.Tag.Apply(errors.Annotate(err, "failed to create a backend task").Err())
			}
			logging.Errorf(ctx, "Give up backend task creation retry after %s", runTaskGiveUpTimeout.String())
		}
		logging.Errorf(ctx, "Backend task creation failure:%s. RunTask request: %+v", err, taskReq)
		dsPutErr := failBuild(ctx, bld.ID, "Backend task creation failure.")
		if dsPutErr != nil {
			return dsPutErr
		}
		return tq.Fatal.Apply(errors.Annotate(err, "failed to create a backend task").Err())
	}
	return nil
}
