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

import { expect } from '@jest/globals';
import { fireEvent, render, screen } from '@testing-library/react';

import {
  Build,
  BuildAgentInputDataRef,
  BuildAgentOutput,
  BuildAgentResolvedDataRef,
  BuildInfra,
  BuildInfraBuildbucket,
} from '../../../../services/buildbucket';
import { BuildPackagesInfo } from './build_packages_info';

describe('BuildPackagesInfo', () => {
  it('build without resolved packages', async () => {
    const buildWithoutOutput = {
      input: { experiments: ['luci.buildbucket.agent.cipd_installation'] },
      infra: {
        buildbucket: {
          agent: {
            input: {
              data: {
                '': {
                  cipd: { specs: [{ package: 'input-pkg', version: 'input-ver' }] },
                } as BuildAgentInputDataRef,
              },
            },
          },
        } as Partial<BuildInfraBuildbucket> as BuildInfraBuildbucket,
      } as Partial<BuildInfra> as BuildInfra,
    } as Partial<Build> as Build;

    render(<BuildPackagesInfo build={buildWithoutOutput} />);
    expect(screen.getByText<HTMLButtonElement>('Resolved').disabled).toBeTruthy();
    expect(screen.getByText('Requested').getAttribute('aria-pressed')).not.toStrictEqual('true');
    expect(screen.queryByText('input-pkg')).toBeNull();

    // Display requested.
    fireEvent.click(screen.getByText('Requested'));
    expect(screen.getByText('Requested').getAttribute('aria-pressed')).toStrictEqual('true');
    expect(screen.queryByText('input-pkg')).not.toBeNull();

    // Click resolved, but it's disabled.
    fireEvent.click(screen.getByText('Resolved'));
    expect(screen.getByText('Requested').getAttribute('aria-pressed')).toStrictEqual('true');
    expect(screen.queryByText('input-pkg')).not.toBeNull();

    // Hide requested.
    fireEvent.click(screen.getByText('Requested'));
    expect(screen.getByText('Requested').getAttribute('aria-pressed')).not.toStrictEqual('true');
    expect(screen.queryByText('input-pkg')).toBeNull();
  });

  it('build with resolved packages', async () => {
    const buildWithoutOutput = {
      input: { experiments: ['luci.buildbucket.agent.cipd_installation'] },
      infra: {
        buildbucket: {
          agent: {
            input: {
              data: {
                '': {
                  cipd: { specs: [{ package: 'input-pkg', version: 'input-ver' }] },
                } as BuildAgentInputDataRef,
              },
            },
            output: {
              resolvedData: {
                '': {
                  cipd: { specs: [{ package: 'output-pkg', version: 'output-ver' }] },
                } as BuildAgentResolvedDataRef,
              },
            } as Partial<BuildAgentOutput> as BuildAgentOutput,
          },
        } as Partial<BuildInfraBuildbucket> as BuildInfraBuildbucket,
      } as Partial<BuildInfra> as BuildInfra,
    } as Partial<Build> as Build;

    render(<BuildPackagesInfo build={buildWithoutOutput} />);
    expect(screen.getByText<HTMLButtonElement>('Resolved').disabled).toBeFalsy();
    expect(screen.getByText('Requested').getAttribute('aria-pressed')).not.toStrictEqual('true');
    expect(screen.getByText('Resolved').getAttribute('aria-pressed')).not.toStrictEqual('true');
    expect(screen.queryByText('input-pkg')).toBeNull();
    expect(screen.queryByText('output-pkg')).toBeNull();

    // Display requested.
    fireEvent.click(screen.getByText('Requested'));
    expect(screen.getByText('Requested').getAttribute('aria-pressed')).toStrictEqual('true');
    expect(screen.getByText('Resolved').getAttribute('aria-pressed')).not.toStrictEqual('true');
    expect(screen.queryByText('input-pkg')).not.toBeNull();
    expect(screen.queryByText('output-pkg')).toBeNull();

    // Display resolved.
    fireEvent.click(screen.getByText('Resolved'));
    expect(screen.getByText('Resolved').getAttribute('aria-pressed')).toStrictEqual('true');
    expect(screen.getByText('Requested').getAttribute('aria-pressed')).not.toStrictEqual('true');
    expect(screen.queryByText('input-pkg')).toBeNull();
    expect(screen.queryByText('output-pkg')).not.toBeNull();

    // Hide resolved.
    fireEvent.click(screen.getByText('Resolved'));
    expect(screen.getByText('Requested').getAttribute('aria-pressed')).not.toStrictEqual('true');
    expect(screen.getByText('Resolved').getAttribute('aria-pressed')).not.toStrictEqual('true');
    expect(screen.queryByText('input-pkg')).toBeNull();
    expect(screen.queryByText('output-pkg')).toBeNull();
  });
});
