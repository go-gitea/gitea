import {createSortable} from '../modules/sortable.ts';
import Sortable, {type SortableEvent, type SortableOptions} from 'sortablejs';
import {POST} from '../modules/fetch.ts';
import {toggleElem} from '../utils/dom.ts';
import {initGroupSelector} from './repo-new.ts';

export function initCommonGroup() {
  if (!document.querySelectorAll('.group').length) {
    return;
  }

  document.querySelector('.group.settings.options #group_name')?.addEventListener('input', function (this: HTMLInputElement) {
    const nameChanged = this.value.toLowerCase() !== this.getAttribute('data-group-name')?.toLowerCase();
    toggleElem('#group-name-change-prompt', nameChanged);
  });

  const form = document.querySelector<HTMLFormElement>('.new-group-form, .ui.form[method="post"]')!;
  initGroupSelector(form);
}

async function moveItem({item, from, to, oldIndex}: SortableEvent): Promise<void> {
  const closestUl = to.nodeName.toLowerCase() === 'ul' ? to : to.closest('ul');
  if (!closestUl || oldIndex === undefined) return;
  const sortable = Sortable.get(closestUl)!;
  const isGroup = Boolean(item.getAttribute('data-is-group'));
  const strs = sortable.toArray();
  const newIndex = Math.max(0, strs.filter((a) => isGroup ?
    a.toLocaleLowerCase().startsWith('group') :
    a.toLocaleLowerCase().startsWith('repo')).indexOf(item.getAttribute('data-sort-id')!));
  const data = {
    newParent: parseInt(to.closest('ul')?.closest('li')?.getAttribute('data-id') || '0'),
    id: parseInt(item.getAttribute('data-id')!),
    newPos: newIndex,
    isGroup,
  };
  let newPath: string;
  try {
    const p = await POST(`${to.getAttribute('data-url')}/items/move`, {
      data,
    });
    const jsonRes = await p.json();
    newPath = jsonRes.newPath;
    const fromItem = from.closest('li');
    const fromLabel = fromItem?.querySelector(':scope > label');
    const itemAnchor = item?.querySelector(':scope > label > a') as HTMLAnchorElement;
    itemAnchor.href = newPath;
    if (from.children.length) {
      fromLabel?.classList?.add('has-children');
    } else {
      fromLabel?.classList?.remove('has-children');
    }
    const toItem = to.closest('li');
    toItem?.querySelector(':scope > label')?.classList.add('has-children');
  } catch (error) {
    console.error(error);
    from.insertBefore(item, from.children[oldIndex]);
  }
}

function onEnd(ev: SortableEvent) {
  const {to} = ev;
  const closestUl = to.nodeName.toLowerCase() === 'ul' ? to : (to.closest('li')?.closest('ul') ?? to.closest('ul'));
  if (!closestUl) return;

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

function initSubgroupSortable(list: HTMLElement | null) {
  if (!list) return;
  createSortable(list, {...baseSortableOptions});
  for (const el of list.querySelectorAll('.expandable-menu-item')) {
    if (!el.getAttribute('data-is-group')) continue;
    const sublist = el.querySelector('ul');
    initSubgroupSortable(sublist);
  }
}

async function initGroupSortable(parentEl: HTMLElement): Promise<void> {
  initSubgroupSortable(parentEl);
}

export function initGroup(): void {
  const mainContainer = document.querySelector('#group-navigation-menu > .sortable') as HTMLElement;
  if (!mainContainer) return;
  initGroupSortable(mainContainer);
}
