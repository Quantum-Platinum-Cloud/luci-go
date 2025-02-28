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

import { afterEach, beforeEach, expect, jest } from '@jest/globals';
import { autorun } from 'mobx';
import { addDisposer, destroy } from 'mobx-state-tree';

import { ANONYMOUS_IDENTITY } from '../libs/auth_state';
import { CacheOption } from '../libs/cached_fn';
import { Build, GetBuildRequest } from '../services/buildbucket';
import { Store, StoreInstance } from '.';

describe('BuildPage', () => {
  describe('cache', () => {
    let store: StoreInstance;
    let getBuildStub: jest.SpiedFunction<(req: GetBuildRequest, cacheOpt?: CacheOption) => Promise<Build>>;

    beforeEach(() => {
      const builderId = { project: 'proj', bucket: 'bucket', builder: 'builder' };
      jest.useFakeTimers();
      store = Store.create({
        authState: { value: { identity: ANONYMOUS_IDENTITY } },
        refreshTime: { value: jest.now() },
      });
      store.buildPage.setParams(builderId, '1');

      getBuildStub = jest.spyOn(store.services.builds!, 'getBuild');
      getBuildStub.mockResolvedValueOnce({ number: 1, id: '2', builder: builderId } as Build);
      getBuildStub.mockResolvedValueOnce({ number: 1, id: '2', builder: builderId } as Build);
    });

    afterEach(() => {
      destroy(store);
      jest.useRealTimers();
    });

    it('should accept cache when first querying build', async () => {
      store.buildPage.build;
      await jest.runAllTimersAsync();
      expect(getBuildStub.mock.calls[0][1]?.acceptCache).not.toBeFalsy();
    });

    it('should not accept cache after calling refresh', async () => {
      store.buildPage.build;
      await jest.runAllTimersAsync();

      await jest.advanceTimersByTimeAsync(10);
      store.refreshTime.refresh();
      store.buildPage.build;
      await jest.runAllTimersAsync();

      expect(getBuildStub.mock.calls[0][1]?.acceptCache).not.toBeFalsy();
      expect(getBuildStub.mock.calls[1][1]?.acceptCache).toBeFalsy();
    });
  });

  describe('params', () => {
    let store: StoreInstance;
    beforeEach(() => {
      jest.useFakeTimers();
      store = Store.create({
        authState: { value: { identity: ANONYMOUS_IDENTITY } },
        refreshTime: { value: jest.now() },
      });
    });

    afterEach(() => {
      destroy(store);
      jest.useRealTimers();
    });

    it('ignore builderIdParam when buildNumOrIdParam is a buildId', async () => {
      store.buildPage.setParams({ project: 'wrong_proj', bucket: 'wrong_bucket', builder: 'wrong_builder' }, 'b123');

      const getBuildStub = jest.spyOn(store.services.builds!, 'getBuild');
      const getBuilderStub = jest.spyOn(store.services.builders!, 'getBuilder');
      const getProjectCfgStub = jest.spyOn(store.services.milo!, 'getProjectCfg');
      const batchCheckPermissionsStub = jest.spyOn(store.services.milo!, 'batchCheckPermissions');

      const builderId = { project: 'proj', bucket: 'bucket', builder: 'builder' };
      getBuildStub.mockResolvedValueOnce({ number: 1, id: '123', builder: builderId } as Build);
      getBuilderStub.mockResolvedValueOnce({ id: builderId, config: {} });
      getProjectCfgStub.mockResolvedValueOnce({});
      batchCheckPermissionsStub.mockResolvedValueOnce({ results: {} });

      addDisposer(
        store,
        autorun(() => {
          store.buildPage.build;
          store.buildPage.builder;
          store.buildPage.customBugLink;
          store.buildPage.canRetry;
        })
      );
      await jest.runAllTimersAsync();

      expect(getBuildStub.mock.calls[0][0].builder).toBeUndefined();
      expect(getBuilderStub.mock.calls[0][0].id).toEqual(builderId);
      expect(getProjectCfgStub.mock.calls[0][0].project).toStrictEqual(builderId.project);
      expect(batchCheckPermissionsStub.mock.calls[0][0].realm).toStrictEqual(
        `${builderId.project}:${builderId.bucket}`
      );
    });
  });
});
