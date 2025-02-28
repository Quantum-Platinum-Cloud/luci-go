// Copyright 2020 The LUCI Authors.
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

package testresults

import (
	"testing"

	"cloud.google.com/go/spanner"
	"google.golang.org/protobuf/proto"
	durpb "google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/server/span"

	"go.chromium.org/luci/resultdb/internal/invocations"
	"go.chromium.org/luci/resultdb/internal/spanutil"
	"go.chromium.org/luci/resultdb/internal/testutil"
	"go.chromium.org/luci/resultdb/internal/testutil/insert"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"

	. "github.com/smartystreets/goconvey/convey"

	. "go.chromium.org/luci/common/testing/assertions"
)

func TestMustParseName(t *testing.T) {
	t.Parallel()

	Convey("MustParseName", t, func() {
		Convey("Parse", func() {
			invID, testID, resultID := MustParseName(
				"invocations/a/tests/ninja:%2F%2Fchrome%2Ftest:foo_tests%2FBarTest.DoBaz/results/result5")
			So(invID, ShouldEqual, "a")
			So(testID, ShouldEqual, "ninja://chrome/test:foo_tests/BarTest.DoBaz")
			So(resultID, ShouldEqual, "result5")
		})

		Convey("Invalid", func() {
			invalidNames := []string{
				"invocations/a/tests/b",
				"invocations/a/tests/b/exonerations/c",
			}
			for _, name := range invalidNames {
				name := name
				So(func() { MustParseName(name) }, ShouldPanic)
			}
		})
	})
}

func TestRead(t *testing.T) {
	Convey(`Read`, t, func() {
		ctx := testutil.SpannerTestContext(t)

		invID := invocations.ID("inv")
		propertiesBytes, err := proto.Marshal(&structpb.Struct{
			Fields: map[string]*structpb.Value{
				"key": structpb.NewStringValue("value"),
			},
		})
		So(err, ShouldBeNil)

		// Insert a TestResult.
		testutil.MustApply(ctx,
			insert.Invocation("inv", pb.Invocation_ACTIVE, nil),
			spanutil.InsertMap("TestResults", map[string]any{
				"InvocationId":    invID,
				"TestId":          "t t",
				"ResultId":        "r",
				"Variant":         pbutil.Variant("k1", "v1", "k2", "v2"),
				"VariantHash":     "deadbeef",
				"CommitTimestamp": spanner.CommitTimestamp,
				"IsUnexpected":    true,
				"Status":          pb.TestStatus_FAIL,
				"RunDurationUsec": 1234567,
				"Properties":      spanutil.Compressed(propertiesBytes),
			}),
		)

		const name = "invocations/inv/tests/t%20t/results/r"
		tr, err := Read(span.Single(ctx), name)
		So(err, ShouldBeNil)
		So(tr, ShouldResembleProto, &pb.TestResult{
			Name:        name,
			TestId:      "t t",
			ResultId:    "r",
			Variant:     pbutil.Variant("k1", "v1", "k2", "v2"),
			Expected:    false,
			Status:      pb.TestStatus_FAIL,
			Duration:    &durpb.Duration{Seconds: 1, Nanos: 234567000},
			VariantHash: "deadbeef",
			Properties: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"key": structpb.NewStringValue("value"),
				},
			},
		})
	})
}
