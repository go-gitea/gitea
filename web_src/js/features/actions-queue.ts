import type SortableType from 'sortablejs';
import {GET, POST} from '../modules/fetch.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {createSortable} from '../modules/sortable.ts';
import {activePageTimerRefresh, createElementFromHTML} from '../utils/dom.ts';
import {Idiomorph} from 'idiomorph';

export function initActionQueueList(): void {
  registerGlobalInitFunc('initActionQueueList', bindActionQueueList);
}

function bindActionQueueList(el: HTMLElement): void {
  // Guards the auto-refresh so it never yanks rows out from under an admin who is dragging or
  // while a reorder POST is still in flight.
  let reordering = false;
  // The queued <tbody> the Sortable instance is bound to. Re-bind when a morph replaces the node.
  let boundTbody: HTMLElement | null = null;
  let sortable: SortableType | null = null;

  async function refresh() {
    const resp = await GET(el.getAttribute('data-queue-refresh-link')!);
    if (!resp.ok || resp.status !== 200) return;
    // The queue rows carry no interactive state, so morph the whole fragment in place.
    // Stable ids on the container/tbody/rows let Idiomorph preserve the Sortable-bound tbody.
    const newEl = createElementFromHTML(await resp.text());
    Idiomorph.morph(el, newEl, {
      morphStyle: 'outerHTML',
      // Leave the filter bar alone: it carries user input (and a fomantic-enhanced select) that a morph would reset.
      callbacks: {beforeNodeMorphed: (node) => !(node instanceof Element && node.id === 'actions-queue-filter')},
    });
    await bindSortable();
  }

  async function persistMove(moveLink: string, item: HTMLElement) {
    try {
      const movedId = item.getAttribute('data-job-id');
      if (!movedId) return;
      // Previous sibling after the drop is the insert anchor (0 = move to head).
      const after = item.previousElementSibling?.getAttribute('data-job-id') ?? '0';
      const resp = await POST(moveLink, {data: new URLSearchParams({id: movedId, after})});
      // On conflict/stale (or any error) restore the server's authoritative order.
      if (!resp.ok) await refresh();
    } catch {
      await refresh();
    } finally {
      // Only re-enable once the server knows the new order, so a second drag can't overtake the first.
      sortable?.option('disabled', false);
      reordering = false;
    }
  }

  // Admins on the first queue page can drag-reorder waiting jobs; the handles + move link only render then.
  async function bindSortable() {
    const moveLink = el.getAttribute('data-queue-move-link');
    const tbody = el.querySelector<HTMLElement>('#actions-queue-tbody');
    if (!moveLink || !tbody) {
      boundTbody = null;
      sortable = null;
      return;
    }
    // Idiomorph preserves the tbody node across refreshes when its id matches, so the existing
    // Sortable binding survives; only (re)create it when the node actually changed.
    if (tbody === boundTbody) return;
    boundTbody = tbody;
    sortable = await createSortable(tbody, {
      handle: '.drag-handle',
      // Table rows don't drag reliably with native HTML5 DnD; use sortable's mouse-based fallback.
      forceFallback: true,
      fallbackOnBody: true,
      onStart() {
        reordering = true;
      },
      onEnd(e) {
        // a drop that ends where it started needs no request, just release the refresh guard
        if (e.oldIndex === e.newIndex) {
          reordering = false;
          return;
        }
        sortable?.option('disabled', true);
        persistMove(moveLink, e.item);
      },
    });
  }

  activePageTimerRefresh({
    interval: () => Number(el.getAttribute('data-queue-refresh-interval')),
    async callback() {
      if (reordering) return;
      await refresh();
    },
  });

  bindSortable();
}
