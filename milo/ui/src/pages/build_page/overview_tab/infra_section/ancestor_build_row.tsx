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

import { getBuildURLPathFromBuildId } from '../../../../libs/url_utils';

export interface AncestorBuildsRowProps {
  readonly ancestorBuildIds?: readonly string[];
}

export function AncestorBuildsRow({ ancestorBuildIds }: AncestorBuildsRowProps) {
  return (
    <tr>
      <td>Ancestor Builds:</td>
      <td>
        {!ancestorBuildIds?.length
          ? 'no ancestor builds'
          : ancestorBuildIds.map((id) => (
              <>
                <a href={getBuildURLPathFromBuildId(id)} target="_blank">
                  {id}
                </a>{' '}
              </>
            ))}
      </td>
    </tr>
  );
}
