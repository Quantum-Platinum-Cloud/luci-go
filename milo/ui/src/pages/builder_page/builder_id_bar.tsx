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

import { getProjectURLPath } from '../../libs/url_utils';
import { BuilderID } from '../../services/buildbucket';

export interface BuilderIdBarProps {
  readonly builderId: BuilderID;
}

export function BuilderIdBar({ builderId }: BuilderIdBarProps) {
  return (
    <div
      css={{
        backgroundColor: 'var(--block-background-color)',
        padding: '6px 16px',
        fontFamily: "'Google Sans', 'Helvetica Neue', sans-serif",
        fontSize: '14px',
        display: 'flex',
      }}
    >
      <div
        css={{
          flex: '0 auto',
          fontSize: '0px',
          '& > *': {
            fontSize: '14px',
          },
        }}
      >
        <span css={{ color: 'var(--light-text-color)' }}>Builder </span>
        <a href={getProjectURLPath(builderId.project)}>{builderId.project}</a>
        <span> / </span>
        <span>{builderId.bucket}</span>
        <span> / </span>
        <span>{builderId.builder}</span>
      </div>
    </div>
  );
}
