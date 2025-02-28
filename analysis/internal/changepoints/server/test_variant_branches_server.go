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

// Package server contains support functions for pb.TestVariantAnalysesServer.
package server

import (
	"context"
	"encoding/hex"
	"fmt"

	"go.chromium.org/luci/analysis/internal/changepoints/inputbuffer"
	tvbr "go.chromium.org/luci/analysis/internal/changepoints/testvariantbranch"
	pb "go.chromium.org/luci/analysis/proto/v1"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/appstatus"
	"go.chromium.org/luci/server/span"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetTestVariantBranch fetches Spanner for test variant branch analysis.
func GetTestVariantBranch(ctx context.Context, tvbk tvbr.TestVariantBranchKey) (*pb.TestVariantBranch, error) {
	txn, cancel := span.ReadOnlyTransaction(ctx)
	defer cancel()
	tvbs, err := tvbr.ReadTestVariantBranches(txn, []tvbr.TestVariantBranchKey{tvbk})
	if err != nil {
		return nil, errors.Annotate(err, "read test variant branch").Err()
	}
	// Should not happen.
	if len(tvbs) != 1 {
		return nil, fmt.Errorf("expected to find only 1 test variant branch. Got %d", len(tvbs))
	}
	// Not found.
	if tvbs[0] == nil {
		return nil, appstatus.Error(codes.NotFound, "analysis not found")
	}
	// Convert to proto.
	analysis := testVariantBranchToProto(tvbs[0])
	return analysis, nil
}

func testVariantBranchToProto(tvb *tvbr.TestVariantBranch) *pb.TestVariantBranch {
	result := &pb.TestVariantBranch{
		Project:           tvb.Project,
		TestId:            tvb.TestID,
		VariantHash:       tvb.VariantHash,
		RefHash:           hex.EncodeToString(tvb.RefHash),
		Variant:           tvb.Variant,
		Ref:               tvb.SourceRef,
		FinalizedSegments: tvb.FinalizedSegments.GetSegments(),
		FinalizingSegment: tvb.FinalizingSegment,
		HotBuffer:         toProtoBuffer(tvb.InputBuffer.HotBuffer),
		ColdBuffer:        toProtoBuffer(tvb.InputBuffer.ColdBuffer),
	}
	return result
}

func toProtoBuffer(history inputbuffer.History) *pb.InputBuffer {
	result := &pb.InputBuffer{
		Length:   int64(len(history.Verdicts)),
		Verdicts: []*pb.PositionVerdict{},
	}
	for _, verdict := range history.Verdicts {
		pv := &pb.PositionVerdict{
			CommitPosition: int64(verdict.CommitPosition),
			Hour:           timestamppb.New(verdict.Hour),
			Runs:           []*pb.PositionVerdict_Run{},
		}
		if verdict.IsSimpleExpected {
			pv.Runs = []*pb.PositionVerdict_Run{
				{
					ExpectedResultCount: 1,
				},
			}
		} else {
			pv.IsExonerated = verdict.Details.IsExonerated
			for _, r := range verdict.Details.Runs {
				pv.Runs = append(pv.Runs, &pb.PositionVerdict_Run{
					ExpectedResultCount:   int64(r.ExpectedResultCount),
					UnexpectedResultCount: int64(r.UnexpectedResultCount),
					IsDuplicate:           r.IsDuplicate,
				})
			}
		}
		result.Verdicts = append(result.Verdicts, pv)
	}
	return result
}
