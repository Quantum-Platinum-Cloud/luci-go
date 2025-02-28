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

import '@material/mwc-button';
import '@material/mwc-icon';
import { css, html } from 'lit';
import { customElement } from 'lit/decorators.js';
import { classMap } from 'lit/directives/class-map.js';
import { repeat } from 'lit/directives/repeat.js';
import { styleMap } from 'lit/directives/style-map.js';
import { makeObservable, observable, reaction } from 'mobx';

import '../../../components/dot_spinner';
import '../../../components/column_header';
import './test_variant_entry';
import { MiloBaseElement } from '../../../components/milo_base';
import { VARIANT_STATUS_CLASS_MAP } from '../../../libs/constants';
import { consumer } from '../../../libs/context';
import { reportErrorAsync } from '../../../libs/error_handler';
import { getTestHistoryURLPath } from '../../../libs/url_utils';
import { urlSetSearchQueryParam } from '../../../libs/utils';
import { getPropKeyLabel, TestVariant, TestVariantStatus } from '../../../services/resultdb';
import { consumeStore, StoreInstance } from '../../../store';
import { consumeInvocationState, InvocationStateInstance } from '../../../store/invocation_state';
import { colorClasses, commonStyles } from '../../../styles/stylesheets';
import { consumeProject } from './context';
import { TestVariantEntryElement } from './test_variant_entry';

export interface VariantGroup {
  readonly def: ReadonlyArray<readonly [string, unknown]>;
  readonly variants: readonly TestVariant[];
  readonly note?: unknown;
}

/**
 * Displays test variants in a table.
 */
@customElement('milo-test-variants-table')
@consumer
export class TestVariantsTableElement extends MiloBaseElement {
  @observable.ref @consumeStore() store!: StoreInstance;
  @observable.ref @consumeInvocationState() invState!: InvocationStateInstance;
  @observable.ref @consumeProject() project: string | undefined;

  constructor() {
    super();
    makeObservable(this);
  }

  private getTvtColumns(columnWidths: readonly number[]) {
    return '24px ' + columnWidths.map((width) => width + 'px').join(' ') + ' 1fr';
  }

  toggleAllVariants(expand: boolean) {
    this.shadowRoot!.querySelectorAll<TestVariantEntryElement>('milo-test-variant-entry').forEach(
      (e) => (e.expanded = expand)
    );
  }

  connectedCallback() {
    super.connectedCallback();

    // When a new test loader is received, load the first page.
    this.addDisposer(
      reaction(
        () => this.invState.loadedFirstPage,
        () => {
          if (this.invState.loadedFirstPage) {
            return;
          }
          reportErrorAsync(this, () => this.invState.loadFirstPage())();
        },
        { fireImmediately: true }
      )
    );

    // Sync column width from the user config.
    this.addDisposer(
      reaction(
        () => this.store.userConfig.tests.columnWidths,
        (columnWidths) => this.invState.setColumnWidths(columnWidths),
        { fireImmediately: true }
      )
    );
  }

  private loadMore = reportErrorAsync(this, () => this.invState.loadNextPage());

  private renderAllVariants() {
    return html`
      ${this.invState.variantGroups.map((group) => this.renderVariantGroup(group))}
      <div id="variant-list-tail">
        ${this.invState.testVariantCount === this.invState.unfilteredTestVariantCount
          ? html`
              Showing ${this.invState.testVariantCount} /
              ${this.invState.unfilteredTestVariantCount}${this.invState.loadedAllTestVariants ? '' : '+'} tests.
            `
          : html`
              Showing
              <i>${this.invState.testVariantCount}</i>
              test${this.invState.testVariantCount === 1 ? '' : 's'} that
              <i>match${this.invState.testVariantCount === 1 ? 'es' : ''} the filter</i>, out of
              <i>${this.invState.unfilteredTestVariantCount}${this.invState.loadedAllTestVariants ? '' : '+'}</i>
              tests.
            `}
        <span
          class="active-text"
          style=${styleMap({
            display: !this.invState.loadedAllTestVariants && this.invState.readyToLoad ? '' : 'none',
          })}
          >${this.renderLoadMore()}</span
        >
      </div>
    `;
  }

  @observable private collapsedVariantGroups = new Set<string>();
  private renderVariantGroup(group: VariantGroup) {
    const groupId = JSON.stringify(group.def);
    const expanded = !this.collapsedVariantGroups.has(groupId);
    return html`
      <div
        class=${classMap({
          expanded,
          empty: group.variants.length === 0,
          'group-header': true,
        })}
        @click=${() => {
          if (expanded) {
            this.collapsedVariantGroups.add(groupId);
          } else {
            this.collapsedVariantGroups.delete(groupId);
          }
        }}
      >
        <mwc-icon class="group-icon">${expanded ? 'expand_more' : 'chevron_right'}</mwc-icon>
        <div>
          <b>${group.variants.length} test variant${group.variants.length === 1 ? '' : 's'}:</b>
          ${group.def.map(
            ([k, v]) =>
              html`<span class="group-kv"
                ><span>${getPropKeyLabel(k)}=</span
                ><span class=${k === 'status' ? VARIANT_STATUS_CLASS_MAP[v as TestVariantStatus] : ''}>${v}</span></span
              >`
          )}
          ${group.note || ''}
        </div>
      </div>
      ${repeat(
        expanded ? group.variants : [],
        (v) => `${v.testId} ${v.variantHash}`,
        (v: TestVariant) => {
          const q = Object.entries(v.variant?.def || {}).map(([dimension, value]) =>
              `V:${encodeURIComponent(dimension)}=${encodeURIComponent(value)}`).join(' ');
          return html`
          <milo-test-variant-entry
            .variant=${v}
            .columnGetters=${this.invState.columnGetters}
            .expanded=${this.invState.testVariantCount === 1}
            .historyUrl=${this.project
              ? urlSetSearchQueryParam(getTestHistoryURLPath(this.project, v.testId), 'q', q)
              : ''}
          ></milo-test-variant-entry>
        `;}
      )}
    `;
  }

  private renderLoadMore() {
    const state = this.invState;
    return html`
      <span style=${styleMap({ display: state.isLoading ?? true ? 'none' : '' })} @click=${() => this.loadMore()}>
        [load more]
      </span>
      <span
        style=${styleMap({
          display: state.isLoading ?? true ? '' : 'none',
          cursor: 'initial',
        })}
      >
        loading <milo-dot-spinner></milo-dot-spinner>
      </span>
    `;
  }

  private tableHeaderEle?: HTMLElement;
  protected updated() {
    this.tableHeaderEle = this.shadowRoot!.getElementById('table-header')!;
  }

  /**
   * Generate a sortByColumn callback for the given column.
   */
  private sortByColumnFn(col: string) {
    return (ascending: boolean) => {
      const matchingKeys = [col, `-${col}`];
      const newKeys = this.invState.sortingKeys.filter((key) => !matchingKeys.includes(key));
      newKeys.unshift((ascending ? '' : '-') + col);
      this.invState.setSortingKeys(newKeys);
    };
  }

  /**
   * Generate a groupByColumn callback for the given column.
   */
  private groupByColumnFn(col: string) {
    return () => {
      this.invState.setColumnKeys(this.invState.columnKeys.filter((key) => key !== col));
      const newKeys = this.invState.groupingKeys.filter((key) => key !== col);
      newKeys.unshift(col);
      this.invState.setGroupingKeys(newKeys);
    };
  }

  protected render() {
    return html`
      <div style="--tvt-columns: ${this.getTvtColumns(this.invState.columnWidths)}">
        <div id="table-header">
          <div><!-- Expand toggle --></div>
          <milo-column-header
            .label=${/* invis char */ '\u2002' + 'S'}
            .tooltip=${'status'}
            .sortByColumn=${this.sortByColumnFn('status')}
            .groupByColumn=${this.groupByColumnFn('status')}
          ></milo-column-header>
          ${this.invState.columnKeys.map(
            (col, i) => html`<milo-column-header
              .label=${getPropKeyLabel(col)}
              .tooltip=${col}
              .resizeColumn=${(delta: number, finalized: boolean) => {
                if (!finalized) {
                  const newColWidths = this.invState.columnWidths.slice();
                  newColWidths[i] += delta;
                  // Update the style directly so lit-element doesn't need to
                  // re-render the component frequently.
                  // Live updating the width of the entire column can cause a bit
                  // of lag when there are many rows. Live updating just the
                  // column header is good enough.
                  this.tableHeaderEle?.style.setProperty('--tvt-columns', this.getTvtColumns(newColWidths));
                  return;
                }

                this.tableHeaderEle?.style.removeProperty('--tvt-columns');
                this.store.userConfig.tests.setColumWidth(col, this.invState.columnWidths[i] + delta);
              }}
              .sortByColumn=${this.sortByColumnFn(col)}
              .groupByColumn=${this.groupByColumnFn(col)}
              .hideColumn=${() => {
                this.invState.setColumnKeys(this.invState.columnKeys.filter((key) => key !== col));
              }}
            ></milo-column-header>`
          )}
          <milo-column-header
            .label=${'Name'}
            .tooltip=${'name'}
            .sortByColumn=${this.sortByColumnFn('name')}
          ></milo-column-header>
        </div>
        <div id="test-variant-list" tabindex="0">${this.renderAllVariants()}</div>
      </div>
    `;
  }

  static styles = [
    commonStyles,
    colorClasses,
    css`
      :host {
        display: block;
        --tvt-top-offset: 0px;
      }

      #table-header {
        display: grid;
        grid-template-columns: 24px var(--tvt-columns);
        grid-gap: 5px;
        line-height: 24px;
        padding: 2px 2px 2px 10px;
        font-weight: bold;
        position: sticky;
        top: var(--tvt-top-offset);
        border-top: 1px solid var(--divider-color);
        border-bottom: 1px solid var(--divider-color);
        background-color: var(--block-background-color);
        z-index: 2;
      }

      #no-invocation {
        padding: 10px;
      }
      #test-variant-list > * {
        padding-left: 10px;
      }
      milo-test-variant-entry {
        margin: 2px 0px;
      }

      #integration-hint {
        border-bottom: 1px solid var(--divider-color);
        padding: 0 0 5px 15px;
      }

      .group-header {
        display: grid;
        grid-template-columns: auto auto 1fr;
        grid-gap: 5px;
        padding: 2px 2px 2px 10px;
        position: sticky;
        top: calc(var(--tvt-top-offset) + 29px);
        font-size: 14px;
        background-color: var(--block-background-color);
        border-top: 1px solid var(--divider-color);
        cursor: pointer;
        line-height: 24px;
        z-index: 1;
      }
      .group-header:first-child {
        top: calc(var(--tvt-top-offset) + 30px);
        border-top: none;
      }
      .group-header.expanded:not(.empty) {
        border-bottom: 1px solid var(--divider-color);
      }
      .group-kv:not(:last-child)::after {
        content: ', ';
      }
      .group-kv > span:first-child {
        color: var(--light-text-color);
      }
      .group-kv > span:nth-child(2) {
        font-weight: 500;
        font-style: italic;
      }

      .inline-icon {
        --mdc-icon-size: 1.2em;
        vertical-align: bottom;
      }

      #variant-list-tail {
        padding: 5px 0 5px 15px;
      }
      #variant-list-tail:not(:first-child) {
        border-top: 1px solid var(--divider-color);
      }
      #load {
        color: var(--active-text-color);
      }
    `,
  ];
}
