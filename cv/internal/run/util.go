// Copyright 2021 The LUCI Authors.
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

package run

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"

	"go.chromium.org/luci/gae/service/datastore"
)

// runHeapKey facilitates heap-based merge of multiple consistently sorted
// ranges of Datastore keys, each range identified by its index.
type runHeapKey struct {
	dsKey   *datastore.Key
	sortKey string
	idx     int
}
type runHeap []runHeapKey

func (r runHeap) Len() int {
	return len(r)
}

func (r runHeap) Less(i int, j int) bool {
	return r[i].sortKey < r[j].sortKey
}

func (r runHeap) Swap(i int, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r *runHeap) Push(x any) {
	*r = append(*r, x.(runHeapKey))
}

func (r *runHeap) Pop() any {
	idx := len(*r) - 1
	v := (*r)[idx]
	(*r)[idx].dsKey = nil // free memory as a good habit.
	*r = (*r)[:idx]
	return v
}

// ComputeCLGroupKey constructs keys for ClGroupKey and the related
// EquivalentClGroupKey.
//
// These are meant to be opaque keys unique to particular set of CLs and
// patchsets for the purpose of grouping together Runs for the same sets of
// patchsets. if isEquivalent is true, then the "min equivalent patchset" is
// used instead of the latest patchset, so that trivial patchsets such as minor
// rebases and CL description updates don't change the key.
func ComputeCLGroupKey(cls []*RunCL, isEquivalent bool) string {
	sort.Slice(cls, func(i, j int) bool {
		// ExternalID includes host and change number but not patchset; but
		// different patchsets of the same CL will never be included in the
		// same list, so sorting on only ExternalID is sufficient.
		return cls[i].ExternalID < cls[j].ExternalID
	})
	h := sha256.New()
	// CL group keys are meant to be opaque keys. We'd like to avoid people
	// depending on CL group key and equivalent CL group key sometimes being
	// equal. We can do this by adding a salt to the hash.
	if isEquivalent {
		h.Write([]byte("equivalent_cl_group_key"))
	}
	separator := []byte{0}
	for i, cl := range cls {
		if i > 0 {
			h.Write(separator)
		}
		h.Write([]byte(cl.Detail.GetGerrit().GetHost()))
		h.Write(separator)
		h.Write([]byte(strconv.FormatInt(cl.Detail.GetGerrit().GetInfo().GetNumber(), 10)))
		h.Write(separator)
		if isEquivalent {
			h.Write([]byte(strconv.FormatInt(int64(cl.Detail.GetMinEquivalentPatchset()), 10)))
		} else {
			h.Write([]byte(strconv.FormatInt(int64(cl.Detail.GetPatchset()), 10)))
		}
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}
