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

package updater

import (
	"go.chromium.org/luci/analysis/internal/analysis"
	"go.chromium.org/luci/analysis/internal/analysis/metrics"
	"go.chromium.org/luci/analysis/internal/bugs"
)

// ExtractResidualImpact extracts the residual impact from a
// cluster. For suggested clusters, residual impact
// is the impact of the cluster after failures that are already
// part of a bug cluster are removed.
func ExtractResidualImpact(c *analysis.Cluster) *bugs.ClusterImpact {
	residualImpact := bugs.ClusterImpact{}
	for id, counts := range c.MetricValues {
		residualImpact[id] = extractMetricImpact(counts)
	}
	return &residualImpact
}

func extractMetricImpact(counts metrics.TimewiseCounts) bugs.MetricImpact {
	return bugs.MetricImpact{
		OneDay:   counts.OneDay.Residual,
		ThreeDay: counts.ThreeDay.Residual,
		SevenDay: counts.SevenDay.Residual,
	}
}

// SetResidualImpact sets the residual impact on a cluster summary.
func SetResidualImpact(cs *analysis.Cluster, impact *bugs.ClusterImpact) {
	for k, v := range *impact {
		cs.MetricValues[k] = replaceResidualImpact(cs.MetricValues[k], v)
	}
}

func replaceResidualImpact(counts metrics.TimewiseCounts, impact bugs.MetricImpact) metrics.TimewiseCounts {
	counts.OneDay.Residual = impact.OneDay
	counts.ThreeDay.Residual = impact.ThreeDay
	counts.SevenDay.Residual = impact.SevenDay
	return counts
}
