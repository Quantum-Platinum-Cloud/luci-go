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

import { expect, jest } from '@jest/globals';
import { fireEvent, render, screen } from '@testing-library/react';

import { Changelist, ChangelistOwnerKind } from '../services/luci_analysis';
import { ChangelistsBadge } from './changelists_badge';
import { ChangelistsTooltipElement } from './changelists_tooltip';
import { ShowTooltipEventDetail } from './tooltip';

const changelists: Changelist[] = [
  { host: 'www.example.com', change: '1234', patchset: 1, ownerKind: ChangelistOwnerKind.Automation },
  { host: 'www.example.com', change: '2345', patchset: 2, ownerKind: ChangelistOwnerKind.Automation },
];

describe('ChangelistsBadge', () => {
  const dispatchEvent = window.dispatchEvent;
  beforeEach(() => {
    jest.useFakeTimers();
  });
  afterEach(() => {
    window.dispatchEvent = dispatchEvent;
    jest.restoreAllMocks();
    jest.useRealTimers();
  });

  it('single changelist', async () => {
    const dispatchEventSpy = jest.spyOn(window, 'dispatchEvent');
    render(<ChangelistsBadge changelists={changelists.slice(0, 1)} />);

    const anchorElement = screen.getByRole<HTMLAnchorElement>('link', { exact: false });
    expect(anchorElement.href).toStrictEqual('https://www.example.com/c/1234/1');
    expect(anchorElement.textContent).toStrictEqual('c/1234/1');

    fireEvent.mouseOver(anchorElement);
    await jest.runAllTimersAsync();

    expect(dispatchEventSpy.mock.calls.length).toStrictEqual(0);
  });

  it('multiple changelists', async () => {
    const dispatchEventSpy = jest.spyOn(window, 'dispatchEvent');
    render(<ChangelistsBadge changelists={changelists} />);

    const anchorElement = screen.getByRole<HTMLAnchorElement>('link', { exact: false });
    expect(anchorElement.href).toStrictEqual('https://www.example.com/c/1234/1');
    expect(anchorElement.textContent).toStrictEqual('c/1234/1, ...');

    fireEvent.mouseOver(anchorElement);
    await jest.runAllTimersAsync();

    expect(dispatchEventSpy.mock.calls.length).toStrictEqual(1);
    const event = dispatchEventSpy.mock.lastCall![0] as CustomEvent<ShowTooltipEventDetail>;
    expect(event.type).toStrictEqual('show-tooltip');
    const tooltip = event.detail.tooltip.getElementsByTagName('milo-changelists-tooltip');
    expect(tooltip.length).toStrictEqual(1);
    expect((tooltip[0] as ChangelistsTooltipElement).changelists).toEqual(changelists);
  });
});
