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

package inputbuffer

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEncodeAndDecode(t *testing.T) {
	Convey(`Encode and decode should return the same result`, t, func() {
		history := History{
			Verdicts: []PositionVerdict{
				{
					CommitPosition:   1345,
					IsSimpleExpected: true,
					Hour:             time.Unix(1000*3600, 0),
				},
				{
					CommitPosition:   1355,
					IsSimpleExpected: false,
					Hour:             time.Unix(1005*3600, 0),
					Details: VerdictDetails{
						IsExonerated: false,
						Runs: []Run{
							{
								ExpectedResultCount:   1,
								UnexpectedResultCount: 2,
								IsDuplicate:           false,
							},
							{
								ExpectedResultCount:   2,
								UnexpectedResultCount: 3,
								IsDuplicate:           true,
							},
						},
					},
				},
				{
					CommitPosition:   1357,
					IsSimpleExpected: true,
					Hour:             time.Unix(1003*3600, 0),
				},
				{
					CommitPosition:   1357,
					IsSimpleExpected: false,
					Hour:             time.Unix(1005*3600, 0),
					Details: VerdictDetails{
						IsExonerated: true,
						Runs: []Run{
							{
								ExpectedResultCount:   0,
								UnexpectedResultCount: 1,
								IsDuplicate:           true,
							},
							{
								ExpectedResultCount:   0,
								UnexpectedResultCount: 1,
								IsDuplicate:           false,
							},
						},
					},
				},
			},
		}

		encoded := EncodeHistory(history)
		decodedHistory, err := DecodeHistory(encoded)
		So(err, ShouldBeNil)
		So(len(decodedHistory.Verdicts), ShouldEqual, 4)
		So(decodedHistory, ShouldResemble, history)
	})

	Convey(`Encode and decode long history should not have error`, t, func() {
		history := History{}
		history.Verdicts = make([]PositionVerdict, 2000)
		for i := 0; i < 2000; i++ {
			history.Verdicts[i] = PositionVerdict{
				CommitPosition:   i,
				IsSimpleExpected: false,
				Hour:             time.Unix(int64(i*3600), 0),
				Details: VerdictDetails{
					IsExonerated: false,
					Runs: []Run{
						{
							ExpectedResultCount:   1,
							UnexpectedResultCount: 2,
							IsDuplicate:           false,
						},
						{
							ExpectedResultCount:   1,
							UnexpectedResultCount: 2,
							IsDuplicate:           false,
						},
						{
							ExpectedResultCount:   1,
							UnexpectedResultCount: 2,
							IsDuplicate:           false,
						},
					},
				},
			}
		}
		encoded := EncodeHistory(history)
		decodedHistory, err := DecodeHistory(encoded)
		So(err, ShouldBeNil)
		So(len(decodedHistory.Verdicts), ShouldEqual, 2000)
		So(decodedHistory, ShouldResemble, history)
	})
}

func TestInputBuffer(t *testing.T) {
	Convey(`Add item to input buffer`, t, func() {
		ib := Buffer{
			HotBufferCapacity:  10,
			ColdBufferCapacity: 100,
		}
		// Insert 9 verdicts into hot buffer.
		ib.InsertVerdict(createTestVerdict(1, 4))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		ib.InsertVerdict(createTestVerdict(2, 2))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		ib.InsertVerdict(createTestVerdict(3, 3))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		ib.InsertVerdict(createTestVerdict(2, 3))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		ib.InsertVerdict(createTestVerdict(4, 5))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		ib.InsertVerdict(createTestVerdict(1, 1))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		ib.InsertVerdict(createTestVerdict(2, 3))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		ib.InsertVerdict(createTestVerdict(7, 8))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		ib.InsertVerdict(createTestVerdict(7, 7))
		So(ib.IsColdBufferDirty, ShouldBeFalse)
		So(len(ib.HotBuffer.Verdicts), ShouldEqual, 9)
		So(ib.HotBuffer.Verdicts, ShouldResemble, []PositionVerdict{
			createTestVerdict(1, 1),
			createTestVerdict(1, 4),
			createTestVerdict(2, 2),
			createTestVerdict(2, 3),
			createTestVerdict(2, 3),
			createTestVerdict(3, 3),
			createTestVerdict(4, 5),
			createTestVerdict(7, 7),
			createTestVerdict(7, 8),
		})
		// Insert the last verdict, expecting a compaction.
		ib.InsertVerdict(createTestVerdict(6, 2))
		So(ib.IsColdBufferDirty, ShouldBeTrue)
		So(len(ib.HotBuffer.Verdicts), ShouldEqual, 0)
		So(len(ib.ColdBuffer.Verdicts), ShouldEqual, 10)
		So(ib.ColdBuffer.Verdicts, ShouldResemble, []PositionVerdict{
			createTestVerdict(1, 1),
			createTestVerdict(1, 4),
			createTestVerdict(2, 2),
			createTestVerdict(2, 3),
			createTestVerdict(2, 3),
			createTestVerdict(3, 3),
			createTestVerdict(4, 5),
			createTestVerdict(6, 2),
			createTestVerdict(7, 7),
			createTestVerdict(7, 8),
		})
	})

	Convey(`Compaction should maintain order`, t, func() {
		ib := Buffer{
			HotBufferCapacity: 5,
			HotBuffer: History{
				Verdicts: []PositionVerdict{
					createTestVerdict(1, 1),
					createTestVerdict(3, 1),
					createTestVerdict(5, 1),
					createTestVerdict(7, 1),
					createTestVerdict(9, 1),
				},
			},
			ColdBufferCapacity: 10,
			ColdBuffer: History{
				Verdicts: []PositionVerdict{
					createTestVerdict(2, 1),
					createTestVerdict(4, 1),
					createTestVerdict(6, 1),
					createTestVerdict(8, 1),
					createTestVerdict(10, 1),
				},
			},
		}

		ib.Compact()
		So(len(ib.HotBuffer.Verdicts), ShouldEqual, 0)
		So(len(ib.ColdBuffer.Verdicts), ShouldEqual, 10)
		So(ib.ColdBuffer.Verdicts, ShouldResemble, []PositionVerdict{
			createTestVerdict(1, 1),
			createTestVerdict(2, 1),
			createTestVerdict(3, 1),
			createTestVerdict(4, 1),
			createTestVerdict(5, 1),
			createTestVerdict(6, 1),
			createTestVerdict(7, 1),
			createTestVerdict(8, 1),
			createTestVerdict(9, 1),
			createTestVerdict(10, 1),
		})
	})

	Convey(`Cold buffer should keep old verdicts after compaction`, t, func() {
		ib := Buffer{
			HotBufferCapacity: 2,
			HotBuffer: History{
				Verdicts: []PositionVerdict{
					createTestVerdict(7, 1),
					createTestVerdict(9, 1),
				},
			},
			ColdBufferCapacity: 5,
			ColdBuffer: History{
				Verdicts: []PositionVerdict{
					createTestVerdict(2, 1),
					createTestVerdict(4, 1),
					createTestVerdict(6, 1),
					createTestVerdict(8, 1),
					createTestVerdict(10, 1),
				},
			},
		}

		ib.Compact()
		So(len(ib.HotBuffer.Verdicts), ShouldEqual, 0)
		So(len(ib.ColdBuffer.Verdicts), ShouldEqual, 7)
		So(ib.ColdBuffer.Verdicts, ShouldResemble, []PositionVerdict{
			createTestVerdict(2, 1),
			createTestVerdict(4, 1),
			createTestVerdict(6, 1),
			createTestVerdict(7, 1),
			createTestVerdict(8, 1),
			createTestVerdict(9, 1),
			createTestVerdict(10, 1),
		})
	})
}

func createTestVerdict(pos int, hour int) PositionVerdict {
	return PositionVerdict{
		CommitPosition:   pos,
		IsSimpleExpected: true,
		Hour:             time.Unix(int64(3600*hour), 0),
	}
}
