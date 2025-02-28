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

import { observer } from 'mobx-react-lite';

import { BuildTagRow } from '../../../components/build_tag_row';
import { useStore } from '../../../store';

export const TagsSection = observer(() => {
  const store = useStore();

  const tags = store.buildPage.build?.data.tags;
  if (!tags) {
    return <></>;
  }

  return (
    <>
      <h3>Tags</h3>
      <table>
        <tbody>
          {tags.map((tag) => (
            <BuildTagRow key={tag.key} tagKey={tag.key} tagValue={tag.value || ''} />
          ))}
        </tbody>
      </table>
    </>
  );
});
