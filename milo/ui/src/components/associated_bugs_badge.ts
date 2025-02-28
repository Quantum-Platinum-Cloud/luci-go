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

import '@material/mwc-menu';
import { css, html, render } from 'lit';
import { customElement } from 'lit/decorators.js';
import { computed, makeObservable, observable, reaction } from 'mobx';

import './associated_bugs_tooltip';
import { AssociatedBug, Cluster } from '../services/luci_analysis';
import { commonStyles } from '../styles/stylesheets';
import { MiloBaseElement } from './milo_base';
import { HideTooltipEventDetail, ShowTooltipEventDetail } from './tooltip';

@customElement('milo-associated-bugs-badge')
export class AssociatedBugsBadgeElement extends MiloBaseElement {
  @observable.ref project!: string;
  @observable.ref clusters!: readonly Cluster[];

  /**
   * Unique bugs in the provided clusters.
   */
  @computed.struct private get uniqueBugs(): readonly AssociatedBug[] {
    const uniqueBugs: AssociatedBug[] = [];
    const seen = new Set<string>();
    for (const cluster of this.clusters) {
      if (!cluster.bug) {
        continue;
      }
      if (seen.has(cluster.bug.url)) {
        continue;
      }
      seen.add(cluster.bug.url);
      uniqueBugs.push(cluster.bug);
    }
    return uniqueBugs;
  }

  constructor() {
    super();
    makeObservable(this);
  }

  connectedCallback() {
    super.connectedCallback();
    this.addDisposer(
      reaction(
        () => this.uniqueBugs.length > 0,
        (shouldDisplay) => this.style.setProperty('display', shouldDisplay ? 'inline-block' : 'none'),
        { fireImmediately: true }
      )
    );
  }

  private renderTooltip() {
    return html`
      <milo-associated-bugs-tooltip .project=${this.project} .clusters=${this.clusters}></milo-associated-bugs-tooltip>
    `;
  }

  protected render() {
    return html`
      <div
        class="badge"
        @mouseover=${() => {
          const tooltip = document.createElement('div');
          render(this.renderTooltip(), tooltip);

          window.dispatchEvent(
            new CustomEvent<ShowTooltipEventDetail>('show-tooltip', {
              detail: {
                tooltip,
                targetRect: this.getBoundingClientRect(),
                gapSize: 2,
              },
            })
          );
        }}
        @mouseout=${() => {
          window.dispatchEvent(new CustomEvent<HideTooltipEventDetail>('hide-tooltip', { detail: { delay: 50 } }));
        }}
      >
        ${this.uniqueBugs.map((b) => b.linkText).join(', ')}
      </div>
    `;
  }

  static styles = [
    commonStyles,
    css`
      .badge {
        display: inline-block;
        margin: 0;
        background-color: #b7b7b7;
        width: 100%;
        box-sizing: border-box;
        overflow: hidden;
        text-overflow: ellipsis;
        vertical-align: sub;
        color: white;
        padding: 0.25em 0.4em;
        font-size: 75%;
        font-weight: 700;
        line-height: 13px;
        text-align: center;
        white-space: nowrap;
        border-radius: 0.25rem;
      }
    `,
  ];
}
