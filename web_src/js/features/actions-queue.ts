import {GET} from '../modules/fetch.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {activePageTimerRefresh, createElementFromHTML} from '../utils/dom.ts';
import {Idiomorph} from 'idiomorph';

export function initActionQueueList(): void {
  registerGlobalInitFunc('initActionQueueList', bindActionQueueList);
}

function bindActionQueueList(el: HTMLElement): void {
  async function refresh() {
    const resp = await GET(el.getAttribute('data-queue-refresh-link')!);
    if (!resp.ok) return;
    // The queue rows carry no interactive state, so morph the whole fragment in place.
    const newEl = createElementFromHTML(await resp.text());
    Idiomorph.morph(el, newEl, {
      morphStyle: 'outerHTML',
      // Leave the filter bar alone: it carries user input (and a fomantic-enhanced select) that a morph would reset.
      callbacks: {beforeNodeMorphed: (node) => !(node instanceof Element && node.id === 'actions-queue-filter')},
    });
  }

  activePageTimerRefresh({
    interval: () => Number(el.getAttribute('data-queue-refresh-interval')),
    callback: refresh,
  });
}
