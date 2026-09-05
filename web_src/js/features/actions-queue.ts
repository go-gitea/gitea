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

  // Admins on the first queue page can drag-reorder waiting jobs; the handles + move link only render then.
  async function bindSortable() {
    const moveLink = el.getAttribute('data-queue-move-link');
    const tbody = el.querySelector<HTMLElement>('#actions-queue-tbody');
    if (!moveLink || !tbody) {
      boundTbody = null;
      return;
    }
    // Idiomorph preserves the tbody node across refreshes when its id matches, so the existing
    // Sortable binding survives; only (re)create it when the node actually changed.
    if (tbody === boundTbody) return;
    boundTbody = tbody;
    await createSortable(tbody, {
      handle: '.drag-handle',
      // Table rows don't drag reliably with native HTML5 DnD; use sortable's mouse-based fallback.
      forceFallback: true,
      fallbackOnBody: true,
      onStart() {
        reordering = true;
      },
      async onEnd(e) { // eslint-disable-line @typescript-eslint/no-misused-promises -- Sortable requires an async callback to persist the reordered job.
        try {
          const movedId = e.item.getAttribute('data-job-id');
          if (!movedId) return;
          // Previous sibling after the drop is the insert anchor (0 = move to head).
          const after = e.item.previousElementSibling?.getAttribute('data-job-id') ?? '0';
          const resp = await POST(moveLink, {data: new URLSearchParams({id: movedId, after})});
          // On conflict/stale (or any error) restore the server's authoritative order.
          if (!resp.ok) await refresh();
        } catch {
          await refresh();
        } finally {
          reordering = false;
        }
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
