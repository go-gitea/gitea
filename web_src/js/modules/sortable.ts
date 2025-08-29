import type {SortableOptions, SortableEvent} from 'sortablejs';
import type SortableType from 'sortablejs';

export async function createSortable(el: Element, opts: {handle?: string} & SortableOptions = {}): Promise<SortableType> {
  // @ts-expect-error: wrong type derived by typescript
  const {Sortable} = await import(/* webpackChunkName: "sortablejs" */'sortablejs');

  return new Sortable(el, {
    animation: 150,
    ghostClass: 'card-ghost',
    onChoose: (e: SortableEvent) => {
      const handle = opts.handle ? e.item.querySelector(opts.handle) : e.item;
      handle.classList.add('tw-cursor-grabbing');
      opts.onChoose?.(e);
    },
    onUnchoose: (e: SortableEvent) => {
      const handle = opts.handle ? e.item.querySelector(opts.handle) : e.item;
      handle.classList.remove('tw-cursor-grabbing');
      opts.onUnchoose?.(e);
    },
    ...opts,
  } satisfies SortableOptions);
}
