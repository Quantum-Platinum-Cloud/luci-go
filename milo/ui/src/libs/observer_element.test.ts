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

import { beforeEach, expect, jest } from '@jest/globals';
import { aTimeout, fixture, fixtureCleanup, html } from '@open-wc/testing-helpers';
import { css } from 'lit';
import { customElement } from 'lit/decorators.js';
import { makeObservable, observable, reaction } from 'mobx';

import { MiloBaseElement } from '../components/milo_base';
import { provider } from './context';
import {
  IntersectionNotifier,
  lazyRendering,
  observer,
  ObserverElement,
  ProgressiveNotifier,
  provideNotifier,
  RenderPlaceHolder,
} from './observer_element';

@customElement('milo-enter-view-observer-notifier-provider-test')
@provider
class EnterViewObserverNotifierProviderElement extends MiloBaseElement {
  @observable.ref
  @provideNotifier()
  notifier = new IntersectionNotifier({ root: this });

  constructor() {
    super();
    makeObservable(this);
  }

  connectedCallback() {
    super.connectedCallback();
    this.addDisposer(
      reaction(
        () => this.notifier,
        (notifier) => {
          // Emulate @property() update.
          this.updated(new Map([['notifier', notifier]]));
        },
        { fireImmediately: true }
      )
    );
  }

  protected render() {
    return html`<slot></slot>`;
  }

  static styles = css`
    :host {
      display: block;
      height: 100px;
      overflow-y: auto;
    }
  `;
}

@customElement('milo-enter-view-observer-test-entry')
@observer
class EnterViewObserverTestEntryElement extends MiloBaseElement implements ObserverElement {
  @observable.ref onEnterCallCount = 0;

  constructor() {
    super();
    makeObservable(this);
  }

  notify() {
    this.onEnterCallCount++;
  }

  protected render() {
    return html`content`;
  }

  static styles = css`
    :host {
      display: block;
      height: 10px;
    }
  `;
}

// jest doesn't support a fully featured intersection observer.
// TODO(weiweilin): change the test to rely on a mocked intersection observer.
describe.skip('enterViewObserver', () => {
  let listView: EnterViewObserverNotifierProviderElement;
  let entries: NodeListOf<EnterViewObserverTestEntryElement>;

  beforeEach(async () => {
    listView = await fixture<EnterViewObserverNotifierProviderElement>(html`
      <milo-enter-view-observer-notifier-provider-test>
        ${new Array(100)
          .fill(0)
          .map(() => html`<milo-enter-view-observer-test-entry></milo-enter-view-observer-test-entry>`)}
      </milo-enter-view-observer-notifier-provider-test>
    `);
    entries = listView.querySelectorAll<EnterViewObserverTestEntryElement>('milo-enter-view-observer-test-entry');
  });

  it('should notify entries in the view.', async () => {
    await aTimeout(20);
    entries.forEach((entry, i) => {
      expect(entry.onEnterCallCount).toStrictEqual(i <= 10 ? 1 : 0);
    });
  });

  it('should notify new entries scrolls into the view.', async () => {
    await aTimeout(20);
    listView.scrollBy(0, 50);
    await aTimeout(20);

    entries.forEach((entry, i) => {
      expect(entry.onEnterCallCount).toStrictEqual(i <= 15 ? 1 : 0);
    });
  });

  it('should re-notify old entries when scrolling back and forth.', async () => {
    await aTimeout(20);
    listView.scrollBy(0, 50);
    await aTimeout(20);
    listView.scrollBy(0, -50);
    await aTimeout(20);

    entries.forEach((entry, i) => {
      expect(entry.onEnterCallCount).toStrictEqual(i <= 15 ? 1 : 0);
    });
  });

  it('different instances can have different notifiers', async () => {
    const notifier1 = new IntersectionNotifier();
    const notifier2 = new IntersectionNotifier();
    const notifier1SubscribeSpy = jest.spyOn(notifier1, 'subscribe');
    const notifier1UnsubscribeSpy = jest.spyOn(notifier1, 'unsubscribe');
    const notifier2SubscribeSpy = jest.spyOn(notifier2, 'subscribe');
    const notifier2UnsubscribeSpy = jest.spyOn(notifier2, 'unsubscribe');
    const provider1 = await fixture(html`
      <milo-enter-view-observer-notifier-provider-test .notifier=${notifier1}>
        <milo-enter-view-observer-test-entry></milo-enter-view-observer-test-entry>
      </milo-enter-view-observer-notifier-provider-test>
    `);
    const provider2 = await fixture(html`
      <milo-enter-view-observer-notifier-provider-test .notifier=${notifier2}>
        <milo-enter-view-observer-test-entry></milo-enter-view-observer-test-entry>
      </milo-enter-view-observer-notifier-provider-test>
    `);

    const entry1 = provider1.querySelector('milo-enter-view-observer-test-entry') as EnterViewObserverTestEntryElement;
    const entry2 = provider2.querySelector('milo-enter-view-observer-test-entry') as EnterViewObserverTestEntryElement;

    expect(notifier1SubscribeSpy.mock.calls.length).toStrictEqual(1);
    expect(notifier1SubscribeSpy.mock.lastCall?.[0]).toStrictEqual(entry1);
    expect(notifier2SubscribeSpy.mock.calls.length).toStrictEqual(1);
    expect(notifier2SubscribeSpy.mock.lastCall?.[0]).toStrictEqual(entry2);

    fixtureCleanup();

    expect(notifier2UnsubscribeSpy.mock.calls.length).toStrictEqual(1);
    expect(notifier2UnsubscribeSpy.mock.lastCall?.[0]).toStrictEqual(entry2);
    expect(notifier1UnsubscribeSpy.mock.calls.length).toStrictEqual(1);
    expect(notifier1UnsubscribeSpy.mock.lastCall?.[0]).toStrictEqual(entry1);
  });

  it('updating observer should works correctly', async () => {
    const notifier1 = new IntersectionNotifier();
    const notifier2 = new IntersectionNotifier();
    const notifier1SubscribeSpy = jest.spyOn(notifier1, 'subscribe');
    const notifier1UnsubscribeSpy = jest.spyOn(notifier1, 'unsubscribe');
    const notifier2SubscribeSpy = jest.spyOn(notifier2, 'subscribe');
    const notifier2UnsubscribeSpy = jest.spyOn(notifier2, 'unsubscribe');

    const provider = await fixture<EnterViewObserverNotifierProviderElement>(html`
      <milo-enter-view-observer-notifier-provider-test .notifier=${notifier1}>
        <milo-enter-view-observer-test-entry></milo-enter-view-observer-test-entry>
      </milo-enter-view-observer-notifier-provider-test>
    `);
    const entry = provider.querySelector('milo-enter-view-observer-test-entry') as EnterViewObserverTestEntryElement;

    expect(notifier1SubscribeSpy.mock.calls.length).toStrictEqual(1);
    expect(notifier1SubscribeSpy.mock.lastCall?.[0]).toStrictEqual(entry);

    provider.notifier = notifier2;
    await aTimeout(20);
    expect(notifier2SubscribeSpy.mock.calls.length).toStrictEqual(1);
    expect(notifier2SubscribeSpy.mock.lastCall?.[0]).toStrictEqual(entry);
    expect(notifier1UnsubscribeSpy.mock.calls.length).toStrictEqual(1);
    expect(notifier1UnsubscribeSpy.mock.lastCall?.[0]).toStrictEqual(entry);

    expect(notifier2UnsubscribeSpy.mock.calls.length).toStrictEqual(1);
    expect(notifier2UnsubscribeSpy.mock.lastCall?.[0]).toStrictEqual(entry);
  });
});

@customElement('milo-lazy-rendering-test-entry')
@lazyRendering
class LazyRenderingElement extends MiloBaseElement implements RenderPlaceHolder {
  constructor() {
    super();
    makeObservable(this);
  }

  renderPlaceHolder() {
    return html`placeholder`;
  }

  protected render() {
    return html`content`;
  }

  static styles = css`
    :host {
      display: block;
      height: 10px;
    }
  `;
}

// jest doesn't support a fully featured intersection observer.
// TODO(weiweilin): change the test to rely on a mocked intersection observer.
describe.skip('lazyRendering', () => {
  let listView: EnterViewObserverNotifierProviderElement;
  let entries: NodeListOf<LazyRenderingElement>;

  beforeEach(async () => {
    listView = await fixture<EnterViewObserverNotifierProviderElement>(html`
      <milo-enter-view-observer-notifier-provider-test>
        ${new Array(100).fill(0).map(() => html`<milo-lazy-rendering-test-entry></milo-lazy-rendering-test-entry>`)}
      </milo-enter-view-observer-notifier-provider-test>
    `);
    entries = listView.querySelectorAll<LazyRenderingElement>('milo-lazy-rendering-test-entry');
  });

  it('should only render content for elements entered the view.', async () => {
    await aTimeout(20);
    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 10 ? 'content' : 'placeholder');
    });
  });

  it('should work with scrolling', async () => {
    await aTimeout(20);
    listView.scrollBy(0, 50);
    await aTimeout(20);

    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 15 ? 'content' : 'placeholder');
    });
  });
});

@customElement('milo-progressive-rendering-test-entry')
@lazyRendering
class ProgressiveRenderingElement extends MiloBaseElement implements RenderPlaceHolder {
  constructor() {
    super();
    makeObservable(this);
  }

  renderPlaceHolder() {
    return html`placeholder`;
  }

  protected render() {
    return html`content`;
  }

  static styles = css`
    :host {
      display: block;
      height: 10px;
    }
  `;
}

@customElement('milo-progressive-rendering-notifier-provider-test')
@provider
class ProgressiveNotifierProviderElement extends MiloBaseElement {
  @provideNotifier() notifier = new ProgressiveNotifier({ batchInterval: 100, batchSize: 10, root: this });

  protected render() {
    return html`<slot></slot>`;
  }

  static styles = css`
    :host {
      display: block;
      height: 100px;
      overflow-y: auto;
    }
  `;
}

// jest doesn't support a fully featured intersection observer.
// TODO(weiweilin): change the test to rely on a mocked intersection observer.
describe.skip('progressiveNotifier', () => {
  let listView: ProgressiveNotifierProviderElement;
  let entries: NodeListOf<ProgressiveRenderingElement>;

  beforeEach(async () => {
    listView = await fixture<ProgressiveNotifierProviderElement>(html`
      <milo-progressive-rendering-notifier-provider-test>
        ${new Array(100)
          .fill(0)
          .map(() => html`<milo-progressive-rendering-test-entry></milo-progressive-rendering-test-entry>`)}
      </milo-progressive-rendering-notifier-provider-test>
    `);
    entries = listView.querySelectorAll<ProgressiveRenderingElement>('milo-progressive-rendering-test-entry');
  });

  it('should only render content for elements entered the view.', async () => {
    await aTimeout(20);
    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 10 ? 'content' : 'placeholder');
    });
  });

  it('should work with scrolling', async () => {
    await aTimeout(20);
    listView.scrollBy(0, 50);
    await aTimeout(20);

    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 15 ? 'content' : 'placeholder');
    });
  });

  it('should notify some of the remaining entries after certain interval', async () => {
    await aTimeout(20);
    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 10 ? 'content' : 'placeholder');
    });

    await aTimeout(150);
    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 20 ? 'content' : 'placeholder');
    });
  });

  it('new notification should reset interval', async () => {
    await aTimeout(20);
    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 10 ? 'content' : 'placeholder');
    });

    await aTimeout(60);
    listView.scrollBy(0, 50);

    await aTimeout(60);
    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 15 ? 'content' : 'placeholder');
    });

    await aTimeout(50);
    entries.forEach((entry, i) => {
      expect(entry.shadowRoot!.textContent).toStrictEqual(i <= 25 ? 'content' : 'placeholder');
    });
  });
});
