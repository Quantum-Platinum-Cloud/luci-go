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

// Package rerun handles rerun for a build.
package rerun

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/bisection/internal/buildbucket"
	"go.chromium.org/luci/bisection/model"
	pb "go.chromium.org/luci/bisection/proto/v1"
	"go.chromium.org/luci/bisection/util/datastoreutil"
)

// TriggerRerun triggers a rerun build for a particular build bucket build and Gitiles commit.
// props is the extra properties to set to the rerun build
// dims is the extra dimension to set to the rerun build
func TriggerRerun(c context.Context, commit *buildbucketpb.GitilesCommit, failedBuildID int64, props map[string]any, dims map[string]string, priority int32) (*buildbucketpb.Build, error) {
	logging.Infof(c, "triggerRerun with commit %s", commit.Id)
	properties, dimensions, err := getRerunPropertiesAndDimensions(c, failedBuildID, props, dims)
	if err != nil {
		logging.Errorf(c, "Failed getRerunPropertiesAndDimension for build %d", failedBuildID)
		return nil, err
	}
	req := &buildbucketpb.ScheduleBuildRequest{
		Builder: &buildbucketpb.BuilderID{
			Project: "chromium",
			Bucket:  "findit",
			Builder: "gofindit-culprit-verification",
		},
		Properties:    properties,
		Dimensions:    dimensions,
		Tags:          getRerunTags(c, failedBuildID),
		GitilesCommit: commit,
		Priority:      priority,
	}
	build, err := buildbucket.ScheduleBuild(c, req)
	if err != nil {
		logging.Errorf(c, "Failed trigger rerun for build %d: %w", failedBuildID, err)
		return nil, err
	}
	logging.Infof(c, "Rerun build %d triggered for build: %d", build.GetId(), failedBuildID)
	return build, nil
}

// getRerunTags returns the build bucket tags for the rerun build
func getRerunTags(c context.Context, bbid int64) []*buildbucketpb.StringPair {
	return []*buildbucketpb.StringPair{
		{
			// analyzed_build_id is the buildbucket ID of the build which we want to rerun.
			Key:   "analyzed_build_id",
			Value: strconv.FormatInt(bbid, 10),
		},
	}
}

// getRerunProperty returns the properties and dimensions for a rerun of a buildID
func getRerunPropertiesAndDimensions(c context.Context, bbid int64, props map[string]any, dims map[string]string) (*structpb.Struct, []*buildbucketpb.RequestedDimension, error) {
	build, err := buildbucket.GetBuild(c, bbid, &buildbucketpb.BuildMask{
		Fields: &fieldmaskpb.FieldMask{
			Paths: []string{"input.properties", "builder", "infra.swarming.task_dimensions"},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get properties for build %d: %w", bbid, err)
	}
	properties, err := getRerunProperties(c, build, props)
	if err != nil {
		return nil, nil, err
	}
	dimens := getRerunDimensions(c, build, dims)
	return properties, dimens, nil
}

func getRerunProperties(c context.Context, build *buildbucketpb.Build, props map[string]any) (*structpb.Struct, error) {
	fields := map[string]any{}
	properties := build.GetInput().GetProperties()
	if properties != nil {
		m := properties.GetFields()
		if builderGroup, ok := m["builder_group"]; ok {
			fields["builder_group"] = builderGroup
			fields["target_builder"] = map[string]string{
				"builder": build.Builder.Builder,
				"group":   builderGroup.GetStringValue(),
			}
		}
		if bootstrapProperties, ok := m["$bootstrap/properties"]; ok {
			fields["$bootstrap/properties"] = bootstrapProperties
		}
	}

	for k, v := range props {
		fields[k] = v
	}

	spb, err := toStructPB(fields)
	if err != nil {
		return nil, fmt.Errorf("cannot convert %v to structpb: %w", fields, err)
	}
	return spb, nil
}

func getRerunDimensions(c context.Context, build *buildbucketpb.Build, dims map[string]string) []*buildbucketpb.RequestedDimension {
	result := []*buildbucketpb.RequestedDimension{}

	// Only copy these dimensions from the analyzed builder to the rerun job request.
	allowedDimensions := map[string]bool{"os": true, "gpu": true}
	if build.GetInfra() != nil && build.GetInfra().GetSwarming() != nil && build.GetInfra().GetSwarming().GetTaskDimensions() != nil {
		dimens := build.GetInfra().GetSwarming().GetTaskDimensions()
		for _, d := range dimens {
			if _, ok := allowedDimensions[d.Key]; ok {
				result = append(result, &buildbucketpb.RequestedDimension{
					Key:   d.Key,
					Value: d.Value,
				})
			}
		}
	}

	// Add extra dimension from dims
	for k, v := range dims {
		result = append(result, &buildbucketpb.RequestedDimension{
			Key:   k,
			Value: v,
		})
	}

	return result
}

// CreateRerunBuildModel creates a CompileRerunBuild (and SingleRerun) in datastore
func CreateRerunBuildModel(c context.Context, build *buildbucketpb.Build, rerunType model.RerunBuildType, suspect *model.Suspect, nsa *model.CompileNthSectionAnalysis, priority int32) (*model.CompileRerunBuild, error) {
	if rerunType == model.RerunBuildType_CulpritVerification && suspect == nil {
		return nil, fmt.Errorf("CreateRerunBuildModel requires suspect when type is CulpritVerification")
	}
	if rerunType == model.RerunBuildType_NthSection && nsa == nil {
		return nil, fmt.Errorf("CreateRerunBuildModel requires nth section analysis when type is NthSection")
	}

	gitilesCommit := *build.GetInput().GetGitilesCommit()
	startTime := build.StartTime.AsTime()
	createTime := build.CreateTime.AsTime()
	rerunBuild := &model.CompileRerunBuild{
		Id: build.GetId(),
		LuciBuild: model.LuciBuild{
			BuildId:    build.GetId(),
			Project:    build.Builder.Project,
			Bucket:     build.Builder.Bucket,
			Builder:    build.Builder.Builder,
			CreateTime: createTime,
			StartTime:  startTime,
			Status:     build.GetStatus(),
			GitilesCommit: buildbucketpb.GitilesCommit{
				Host:    gitilesCommit.Host,
				Project: gitilesCommit.Project,
				Ref:     gitilesCommit.Ref,
				Id:      gitilesCommit.Id,
			},
		},
	}
	err := datastore.Put(c, rerunBuild)
	if err != nil {
		logging.Errorf(c, "Error in creating CompileRerunBuild model for build %d", build.GetId())
		return nil, err
	}

	// Create the first SingleRerun for CompileRerunBuild
	// It will be updated when we receive updates from recipe
	singleRerun := &model.SingleRerun{
		RerunBuild: datastore.KeyForObj(c, rerunBuild),
		Status:     pb.RerunStatus_RERUN_STATUS_IN_PROGRESS,
		GitilesCommit: buildbucketpb.GitilesCommit{
			Host:    gitilesCommit.Host,
			Project: gitilesCommit.Project,
			Ref:     gitilesCommit.Ref,
			Id:      gitilesCommit.Id,
		},
		CreateTime: createTime,
		StartTime:  startTime,
		Type:       rerunType,
		Priority:   priority,
	}

	if rerunType == model.RerunBuildType_CulpritVerification {
		singleRerun.Analysis = suspect.ParentAnalysis.Parent()
		singleRerun.Suspect = datastore.KeyForObj(c, suspect)
	}
	if rerunType == model.RerunBuildType_NthSection {
		singleRerun.Analysis = nsa.ParentAnalysis
		singleRerun.NthSectionAnalysis = datastore.KeyForObj(c, nsa)
	}

	err = datastore.Put(c, singleRerun)
	if err != nil {
		logging.Errorf(c, "Error in creating SingleRerun model for build %d", build.GetId())
		return nil, err
	}

	return rerunBuild, nil
}

// UpdateRerunStartTime updates the start time of rerun builds (when we received buildbucket pubsub messages)
func UpdateRerunStartTime(c context.Context, bbid int64) error {
	logging.Infof(c, "UpdateRerunStartTime for build %d", bbid)
	rerunModel := &model.CompileRerunBuild{
		Id: bbid,
	}

	err := datastore.Get(c, rerunModel)
	if err == datastore.ErrNoSuchEntity {
		// There are cases where we cannot find datastore entries, like
		// luci-bisection-dev receives pubsub message for a prod run
		// In this case, just log and return nil
		logging.Warningf(c, "Couldn't find rerun to UpdateRerunStartTime: %d", bbid)
		return nil
	}
	if err != nil {
		return errors.Annotate(err, "couldn't get rerun model %d", bbid).Err()
	}

	lastRerun, err := datastoreutil.GetLastRerunForRerunBuild(c, rerunModel)
	if err != nil {
		return errors.Annotate(err, "failed getting last rerun for build %d", rerunModel.Id).Err()
	}

	build, err := buildbucket.GetBuild(c, bbid, &buildbucketpb.BuildMask{
		Fields: &fieldmaskpb.FieldMask{
			Paths: []string{"id", "builder", "start_time", "status"},
		},
	})
	if err != nil {
		return errors.Annotate(err, "couldn't get build %d", bbid).Err()
	}

	startTime := build.StartTime.AsTime()

	err = datastore.RunInTransaction(c, func(ctx context.Context) error {
		e := datastore.Get(c, rerunModel)
		if e != nil {
			return e
		}
		rerunModel.StartTime = startTime
		rerunModel.Status = build.Status
		return datastore.Put(c, rerunModel)
	}, nil)

	if err != nil {
		return errors.Annotate(err, "couldn't save rerun model %d", bbid).Err()
	}

	err = datastore.RunInTransaction(c, func(ctx context.Context) error {
		e := datastore.Get(c, lastRerun)
		if e != nil {
			return e
		}
		lastRerun.StartTime = startTime
		return datastore.Put(c, lastRerun)
	}, nil)

	if err != nil {
		return errors.Annotate(err, "failed saving last rerun for build %d", rerunModel.Id).Err()
	}
	return nil
}

// TODO (nqmtuan): Move this into a helper class if it turns out we need to use
// it for more than one place
// toStructPB convert an any s to structpb.Struct, as long as s is marshallable.
// s can be a general Go type, structpb.Struct type, or mixed.
// For example, s can be a map of mixed type, like
// {"key1": "val1", "key2": structpb.NewStringValue("val2")}
func toStructPB(s any) (*structpb.Struct, error) {
	// We used json as an intermediate format to convert
	j, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(j, &m); err != nil {
		return nil, err
	}
	return structpb.NewStruct(m)
}
