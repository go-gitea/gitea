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
  const newFilter = newEl.querySelector('#actions-queue-filter')!;
  const list = el.querySelector('#actions-queue-list')!;
  const newList = newEl.querySelector('#actions-queue-list')!;

  for (const attr of newEl.attributes) el.setAttribute(attr.name, attr.value);
  Idiomorph.morph(list, newList, {morphStyle: 'outerHTML'});
  if (!filter.querySelector('.dropdown.active') && !filter.contains(document.activeElement)) {
    filter.replaceWith(newFilter);
  }
}
