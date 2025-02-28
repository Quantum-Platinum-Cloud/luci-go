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

import { getSwarmingTaskURL } from '../../../../libs/url_utils';
import { BuildInfraSwarming } from '../../../../services/buildbucket';

export interface SwarmingTaskRowProps {
  readonly swarming: BuildInfraSwarming;
}

export function SwarmingTaskRow({ swarming }: SwarmingTaskRowProps) {
  return (
    <tr>
      <td>Swarming Task:</td>
      <td>
        {swarming.taskId ? (
          <a href={getSwarmingTaskURL(swarming.hostname, swarming.taskId)} target="_blank">
            {swarming.taskId}
          </a>
        ) : (
          'N/A'
        )}
      </td>
    </tr>
  );
}
