export async function createSortable(...args) {
  const {Sortable} = await import(/* webpackChunkName: "sortablejs" */'sortablejs');
  return new Sortable(...args);
}
