export async function createSortable(el, opts = {}) {
  const {Sortable} = await import(/* webpackChunkName: "sortablejs" */'sortablejs');

  return new Sortable(el, {
    animation: 150,
    ghostClass: 'card-ghost',
    onChoose: (e) => {
      const handle = opts.handle ? e.item.querySelector(opts.handle) : e.item;
      handle.classList.add('tw-cursor-grabbing');
      opts.onChoose?.(e);
    },
    onUnchoose: (e) => {
      const handle = opts.handle ? e.item.querySelector(opts.handle) : e.item;
      handle.classList.remove('tw-cursor-grabbing');
      opts.onUnchoose?.(e);
    },
    ...opts,
  });
}
