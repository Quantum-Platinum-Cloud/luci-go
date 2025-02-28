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

import { expect } from '@jest/globals';

import { getBuilderURLPath, getTestHistoryURLPath } from './url_utils';

describe('getBuilderURLPath', () => {
  it('should encode the builder', () => {
    const url = getBuilderURLPath({ project: 'testproject', bucket: 'testbucket', builder: 'test builder' });
    expect(url).toStrictEqual('/p/testproject/builders/testbucket/test%20builder');
  });
});

describe('getTestHisotryURLPath', () => {
  it('should encode the test ID', () => {
    const url = getTestHistoryURLPath('testproject', 'test/id');
    expect(url).toStrictEqual('/ui/test/testproject/test%2Fid');
  });
});
