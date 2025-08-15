import {createSortable} from '../modules/sortable.ts';
import Sortable, {type SortableEvent, type SortableOptions} from 'sortablejs';
import {POST} from '../modules/fetch.ts';
import {toggleElem} from '../utils/dom.ts';
export function initCommonGroup() {
  if (!document.querySelectorAll('.group').length) {
    return;
  }

  document.querySelector('.group.settings.options #group_name')?.addEventListener('input', function () {
    const nameChanged = this.value.toLowerCase() !== this.getAttribute('data-group-name').toLowerCase();
    toggleElem('#group-name-change-prompt', nameChanged);
  });
}

async function moveItem({item, from, to, oldIndex}: SortableEvent): Promise<void> {
  const closestUl = to.nodeName.toLowerCase() === 'ul' ? to : to.closest('ul');
  const sortable = Sortable.get(closestUl);
  const isGroup = Boolean(item.getAttribute('data-is-group'));
  const strs = sortable.toArray();
  const newIndex = Math.max(1, strs.filter((a) => isGroup ?
    a.toLocaleLowerCase().startsWith('group') :
    a.toLocaleLowerCase().startsWith('repo')).indexOf(item.getAttribute('data-sort-id')) + 1);
  const data = {
    newParent: parseInt(to.closest('ul').closest('li')?.getAttribute('data-id') || '0'),
    id: parseInt(item.getAttribute('data-id')),
    newPos: newIndex,
    isGroup,
  };
  try {
    await POST(`${to.getAttribute('data-url')}/items/move`, {
      data,
    });
  } catch (error) {
    console.error(error);
    from.insertBefore(item, from.children[oldIndex]);
  }
}

function idSortFn(a: string, b: string): number {
  return parseInt(a.split('-')[2]) - parseInt(b.split('-')[2]);
}

function onEnd(ev: SortableEvent) {
  const {to} = ev;
  const closestUl = to.nodeName.toLowerCase() === 'ul' ? to : (to.closest('li')?.closest('ul') ?? to.closest('ul'));
  const sortable = Sortable.get(closestUl);
  const strs = sortable.toArray();
  const groups = strs.filter((a) => a.toLocaleLowerCase().startsWith('group'));
  const repos = strs.filter((a) => a.toLocaleLowerCase().startsWith('repo'));
  const newArr = [...groups.toSorted(idSortFn), ...repos.toSorted(idSortFn)];
  sortable.sort(newArr, true);
  moveItem(ev);
}

const baseSortableOptions: SortableOptions = {
  group: {
    name: 'group',
    put(to, from, drag, ev) {
      console.debug('to', to);
      console.debug('from', from);
      console.debug('drag', drag);
      console.debug('ev', ev);
      const closestUl = to.el.nodeName.toLowerCase() === 'ul' ? to.el : to.el.closest('ul');
      console.debug('put this');
      return Boolean(closestUl?.getAttribute('data-is-group') ?? true);
    },
    pull: true,
  },
  dataIdAttr: 'data-sort-id',
  draggable: '.expandable-menu-item',
  delayOnTouchOnly: true,
  delay: 500,
  // onMove: beforeMove,
  onEnd,
  emptyInsertThreshold: 25,
};

function initSubgroupSortable(list: Element) {
  createSortable(list, {...baseSortableOptions});
  for (const el of list.querySelectorAll('.expandable-menu-item')) {
    if (!el.getAttribute('data-is-group')) continue;
    const sublist = el.querySelector('ul');
    initSubgroupSortable(sublist);
  }
}

async function initGroupSortable(parentEl: Element): Promise<void> {
  initSubgroupSortable(parentEl);
}

export function initGroup(): void {
  const mainContainer = document.querySelector('#group-navigation-menu > .sortable');
  if (!mainContainer) return;
  initGroupSortable(mainContainer);
}
