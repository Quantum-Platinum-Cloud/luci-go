// Copyright 2023 The LUCI Authors.
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

package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/smartystreets/goconvey/convey"
	"go.chromium.org/luci/analysis/internal/changepoints/inputbuffer"
	tvbr "go.chromium.org/luci/analysis/internal/changepoints/testvariantbranch"
	"go.chromium.org/luci/analysis/internal/testutil"
	pb "go.chromium.org/luci/analysis/proto/v1"
	. "go.chromium.org/luci/common/testing/assertions"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTestVariantAnalysesServer(t *testing.T) {
	Convey("TestVariantAnalysesServer", t, func() {
		ctx := testutil.IntegrationTestContext(t)

		server := NewTestVariantBranchesServer()

		Convey("permission denied", func() {
			ctx = auth.WithState(ctx, &authtest.FakeState{
				Identity: "anonymous:anonymous",
			})
			req := &pb.GetTestVariantBranchRequest{}
			res, err := server.Get(ctx, req)
			So(err, ShouldNotBeNil)
			So(err, ShouldHaveGRPCStatus, codes.PermissionDenied)
			So(res, ShouldBeNil)
		})

		Convey("invalid request", func() {
			ctx = adminContext(ctx)
			req := &pb.GetTestVariantBranchRequest{
				Name: "Project/abc/xyz",
			}
			res, err := server.Get(ctx, req)
			So(err, ShouldNotBeNil)
			So(err, ShouldHaveGRPCStatus, codes.InvalidArgument)
			So(res, ShouldBeNil)
		})

		Convey("not found", func() {
			ctx = adminContext(ctx)
			req := &pb.GetTestVariantBranchRequest{
				Name: "projects/project/tests/test/variants/abababababababab/refs/abababababababab",
			}
			res, err := server.Get(ctx, req)
			So(err, ShouldNotBeNil)
			So(err, ShouldHaveGRPCStatus, codes.NotFound)
			So(res, ShouldBeNil)
		})

		Convey("invalid ref_hash", func() {
			ctx = adminContext(ctx)
			req := &pb.GetTestVariantBranchRequest{
				Name: "projects/project/tests/this//is/a/test/variants/abababababababab/refs/abababababababgh",
			}
			res, err := server.Get(ctx, req)
			So(err, ShouldNotBeNil)
			So(err, ShouldHaveGRPCStatus, codes.InvalidArgument)
			So(res, ShouldBeNil)
		})

		Convey("ok", func() {
			ctx = adminContext(ctx)
			// Insert test variant branch to Spanner.
			tvb := &tvbr.TestVariantBranch{
				IsNew:       true,
				Project:     "project",
				TestID:      "this//is/a/test",
				VariantHash: "0123456789abcdef",
				RefHash:     []byte("ref_hash"),
				SourceRef: &pb.SourceRef{
					System: &pb.SourceRef_Gitiles{
						Gitiles: &pb.GitilesRef{
							Host:    "host",
							Project: "proj",
							Ref:     "ref",
						},
					},
				},
				Variant: &pb.Variant{
					Def: map[string]string{
						"k": "v",
					},
				},
				InputBuffer: &inputbuffer.Buffer{
					HotBuffer: inputbuffer.History{
						Verdicts: []inputbuffer.PositionVerdict{
							{
								CommitPosition:   20,
								IsSimpleExpected: true,
								Hour:             time.Unix(3600, 0),
							},
						},
					},
					ColdBuffer: inputbuffer.History{
						Verdicts: []inputbuffer.PositionVerdict{
							{
								CommitPosition: 30,
								Hour:           time.Unix(7200, 0),
								Details: inputbuffer.VerdictDetails{
									IsExonerated: true,
									Runs: []inputbuffer.Run{
										{
											ExpectedResultCount:   3,
											UnexpectedResultCount: 4,
											IsDuplicate:           true,
										},
										{
											ExpectedResultCount:   2,
											UnexpectedResultCount: 1,
										},
									},
								},
							},
						},
					},
				},
				FinalizingSegment: &pb.Segment{
					State:                        pb.SegmentState_FINALIZING,
					HasStartChangepoint:          true,
					StartPosition:                100,
					StartHour:                    timestamppb.New(time.Unix(3600, 0)),
					StartPositionLowerBound_99Th: 95,
					StartPositionUpperBound_99Th: 105,
					FinalizedCounts: &pb.Counts{
						UnexpectedResults: 1,
					},
				},
				FinalizedSegments: &pb.Segments{
					Segments: []*pb.Segment{
						{
							State:                        pb.SegmentState_FINALIZED,
							StartPosition:                50,
							StartHour:                    timestamppb.New(time.Unix(3600, 0)),
							StartPositionLowerBound_99Th: 45,
							StartPositionUpperBound_99Th: 55,
							FinalizedCounts: &pb.Counts{
								UnexpectedResults: 2,
							},
						},
					},
				},
			}
			mutation, err := tvb.ToMutation()
			So(err, ShouldBeNil)
			testutil.MustApply(ctx, mutation)

			hexStr := "7265665f68617368" // hex string of "ref_hash".
			req := &pb.GetTestVariantBranchRequest{
				Name: "projects/project/tests/this//is/a/test/variants/0123456789abcdef/refs/7265665f68617368",
			}
			res, err := server.Get(ctx, req)
			So(err, ShouldBeNil)
			diff := cmp.Diff(res, &pb.TestVariantBranch{
				Project:           "project",
				TestId:            "this//is/a/test",
				VariantHash:       "0123456789abcdef",
				RefHash:           hexStr,
				Variant:           tvb.Variant,
				Ref:               tvb.SourceRef,
				FinalizingSegment: tvb.FinalizingSegment,
				FinalizedSegments: tvb.FinalizedSegments.Segments,
				HotBuffer: &pb.InputBuffer{
					Length: 1,
					Verdicts: []*pb.PositionVerdict{
						{
							CommitPosition: 20,
							Hour:           timestamppb.New(time.Unix(3600, 0)),
							Runs: []*pb.PositionVerdict_Run{
								{
									ExpectedResultCount: 1,
								},
							},
						},
					},
				},
				ColdBuffer: &pb.InputBuffer{
					Length: 1,
					Verdicts: []*pb.PositionVerdict{
						{
							CommitPosition: 30,
							Hour:           timestamppb.New(time.Unix(7200, 0)),
							IsExonerated:   true,
							Runs: []*pb.PositionVerdict_Run{
								{
									ExpectedResultCount:   3,
									UnexpectedResultCount: 4,
									IsDuplicate:           true,
								},
								{
									ExpectedResultCount:   2,
									UnexpectedResultCount: 1,
								},
							},
						},
					},
				},
			}, cmp.Comparer(proto.Equal))
			So(diff, ShouldEqual, "")
		})
	})
}

func adminContext(ctx context.Context) context.Context {
	return auth.WithState(ctx, &authtest.FakeState{
		Identity:       "user:admin@example.com",
		IdentityGroups: []string{"service-luci-analysis-admins"},
	})
}
