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
    updateActionQueueList(el, newEl);
  }

  activePageTimerRefresh({
    interval: () => Number(el.getAttribute('data-queue-refresh-interval')),
    callback: refresh,
  });
}

export function updateActionQueueList(el: HTMLElement, newEl: Element): void {
  const filter = el.querySelector('#actions-queue-filter')!;
  // Preserve filter interaction while the job rows continue to refresh.
  const deferFilters = filter.querySelector('.dropdown.active') || filter.contains(document.activeElement);

  const newFilter = newEl.querySelector('#actions-queue-filter')!.cloneNode(true);
  Idiomorph.morph(el, newEl, {
    morphStyle: 'outerHTML',
    callbacks: {beforeNodeMorphed: (node) => node !== filter},
  });
  if (!deferFilters) filter.replaceWith(newFilter); // The global observer initializes fresh dropdown nodes.
}
