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

import { Box, ToggleButton, ToggleButtonGroup } from '@mui/material';
import { Fragment, useState } from 'react';

import { MiloLink } from '../../../../components/link';
import { getCipdLink } from '../../../../libs/build_utils';
import { BUILD_STATUS_CLASS_MAP, BUILD_STATUS_DISPLAY_MAP } from '../../../../libs/constants';
import { Build } from '../../../../services/buildbucket';

export interface BuildPackagesInfoProps {
  readonly build: Build;
}

export function BuildPackagesInfo({ build }: BuildPackagesInfoProps) {
  const [displayType, setDisplayType] = useState<null | 'requested' | 'resolved'>(null);
  const experiments = build.input?.experiments;
  const agent = build.infra?.buildbucket?.agent;
  if (!experiments?.includes('luci.buildbucket.agent.cipd_installation') || !agent) {
    return <></>;
  }

  const data = displayType === 'requested' ? agent.input.data : agent.output?.resolvedData || {};

  return (
    <>
      {agent.output?.summaryHtml && (
        <Box
          sx={{ padding: '10px', marginBottom: '10px', clear: 'both', overlapWrap: 'break-word' }}
          className={`${BUILD_STATUS_CLASS_MAP[agent.output.status]}-bg`}
          dangerouslySetInnerHTML={{ __html: agent.output.summaryHtml }}
        />
      )}
      {/*Use table instead of MUI or CSS grid to be consistent with other
       * sessions in the overview tab.
       */}
      <table>
        <tbody>
          <tr>
            <td>Status:</td>
            <td>
              <span className={agent.output ? BUILD_STATUS_CLASS_MAP[agent.output.status] : ''}>
                {agent.output ? BUILD_STATUS_DISPLAY_MAP[agent.output.status] : 'N/A'}
              </span>
            </td>
          </tr>
          <tr>
            <td>Agent Platform:</td>
            <td>{agent.output?.agentPlatform || 'N/A'}</td>
          </tr>
          <tr>
            <td>Download Duration:</td>
            <td>{agent.output?.totalDuration || 'N/A'}</td>
          </tr>
          <tr>
            <td>$ServiceURL:</td>
            <td>
              <a href="https://chrome-infra-packages.appspot.com" target="_blank">
                https://chrome-infra-packages.appspot.com
              </a>
            </td>
          </tr>
          <tr>
            <td>Show Packages:</td>
            <td>
              <ToggleButtonGroup
                exclusive
                value={displayType}
                onChange={(_, newValue) => setDisplayType(newValue)}
                size="small"
              >
                <ToggleButton value="requested">Requested</ToggleButton>
                <ToggleButton value="resolved" disabled={!agent.output?.resolvedData}>
                  Resolved
                </ToggleButton>
              </ToggleButtonGroup>
            </td>
          </tr>
        </tbody>
      </table>
      {displayType && (
        <Box sx={{ overflowX: 'scroll', whiteSpace: 'nowrap' }}>
          <table css={{ borderSpacing: '10px 0' }}>
            <tbody>
              {Object.entries(data).map(([dir, ref]) => {
                if (!ref.cipd.specs.length) {
                  return <Fragment key={dir}></Fragment>;
                }
                return (
                  <Fragment key={dir}>
                    <tr css={{ height: '10px' }}>
                      <td colSpan={2}></td>
                    </tr>
                    {dir && (
                      <tr>
                        <td colSpan={2}>@Subdir {dir}</td>
                      </tr>
                    )}
                    {ref.cipd.specs.map((spec) => (
                      <tr key={spec.package}>
                        <td>{spec.package}</td>
                        <td>
                          {displayType === 'resolved' ? (
                            <MiloLink link={getCipdLink(spec.package, spec.version)} target="_blank" />
                          ) : (
                            spec.version
                          )}
                        </td>
                      </tr>
                    ))}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </Box>
      )}
    </>
  );
}
