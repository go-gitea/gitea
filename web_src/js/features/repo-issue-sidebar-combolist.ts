import {fomanticQuery} from '../modules/fomantic/base.ts';
import {POST} from '../modules/fetch.ts';
import {queryElemChildren, queryElems, toggleElem} from '../utils/dom.ts';

// if there are draft comments, confirm before reloading, to avoid losing comments
function issueSidebarReloadConfirmDraftComment() {
  const commentTextareas = [
    document.querySelector<HTMLTextAreaElement>('.edit-content-zone:not(.tw-hidden) textarea'),
    document.querySelector<HTMLTextAreaElement>('#comment-form textarea'),
  ];
  for (const textarea of commentTextareas) {
    // Most users won't feel too sad if they lose a comment with 10 chars, they can re-type these in seconds.
    // But if they have typed more (like 50) chars and the comment is lost, they will be very unhappy.
    if (textarea && textarea.value.trim().length > 10) {
      textarea.parentElement.scrollIntoView();
      if (!window.confirm('Page will be reloaded, but there are draft comments. Continuing to reload will discard the comments. Continue?')) {
        return;
      }
      break;
    }
  }
  window.location.reload();
}

class IssueSidebarComboList {
  updateUrl: string;
  updateAlgo: string;
  selectionMode: string;
  elDropdown: HTMLElement;
  elList: HTMLElement;
  elComboValue: HTMLInputElement;
  initialValues: string[];

  constructor(private container: HTMLElement) {
    this.updateUrl = this.container.getAttribute('data-update-url');
    this.updateAlgo = container.getAttribute('data-update-algo');
    this.selectionMode = container.getAttribute('data-selection-mode');
    if (!['single', 'multiple'].includes(this.selectionMode)) throw new Error(`Invalid data-update-on: ${this.selectionMode}`);
    if (!['diff', 'all'].includes(this.updateAlgo)) throw new Error(`Invalid data-update-algo: ${this.updateAlgo}`);
    this.elDropdown = container.querySelector<HTMLElement>(':scope > .ui.dropdown');
    this.elList = container.querySelector<HTMLElement>(':scope > .ui.list');
    this.elComboValue = container.querySelector<HTMLInputElement>(':scope > .combo-value');
  }

  collectCheckedValues() {
    return Array.from(this.elDropdown.querySelectorAll('.menu > .item.checked'), (el) => el.getAttribute('data-value'));
  }

  updateUiList(changedValues: Array<string>) {
    const elEmptyTip = this.elList.querySelector('.item.empty-list');
    queryElemChildren(this.elList, '.item:not(.empty-list)', (el) => el.remove());
    for (const value of changedValues) {
      const el = this.elDropdown.querySelector<HTMLElement>(`.menu > .item[data-value="${CSS.escape(value)}"]`);
      if (!el) continue;
      const listItem = el.cloneNode(true) as HTMLElement;
      queryElems(listItem, '.item-check-mark, .item-secondary-info', (el) => el.remove());
      this.elList.append(listItem);
    }
    const hasItems = Boolean(this.elList.querySelector('.item:not(.empty-list)'));
    toggleElem(elEmptyTip, !hasItems);
  }

  async updateToBackend(changedValues: Array<string>) {
    if (this.updateAlgo === 'diff') {
      for (const value of this.initialValues) {
        if (!changedValues.includes(value)) {
          await POST(this.updateUrl, {data: new URLSearchParams({action: 'detach', id: value})});
        }
      }
      for (const value of changedValues) {
        if (!this.initialValues.includes(value)) {
          await POST(this.updateUrl, {data: new URLSearchParams({action: 'attach', id: value})});
        }
      }
    } else {
      await POST(this.updateUrl, {data: new URLSearchParams({id: changedValues.join(',')})});
    }
    issueSidebarReloadConfirmDraftComment();
  }

  async doUpdate() {
    const changedValues = this.collectCheckedValues();
    if (this.initialValues.join(',') === changedValues.join(',')) return;
    this.updateUiList(changedValues);
    if (this.updateUrl) await this.updateToBackend(changedValues);
    this.initialValues = changedValues;
  }

  async onChange() {
    if (this.selectionMode === 'single') {
      await this.doUpdate();
      fomanticQuery(this.elDropdown).dropdown('hide');
    }
  }

  async onItemClick(e: Event) {
    const elItem = (e.target as HTMLElement).closest('.item');
    if (!elItem) return;
    e.preventDefault();
    if (elItem.hasAttribute('data-can-change') && elItem.getAttribute('data-can-change') !== 'true') return;

    if (elItem.matches('.clear-selection')) {
      queryElems(this.elDropdown, '.menu > .item', (el) => el.classList.remove('checked'));
      this.elComboValue.value = '';
      this.onChange();
      return;
    }

    const scope = elItem.getAttribute('data-scope');
    if (scope) {
      // scoped items could only be checked one at a time
      const elSelected = this.elDropdown.querySelector<HTMLElement>(`.menu > .item.checked[data-scope="${CSS.escape(scope)}"]`);
      if (elSelected === elItem) {
        elItem.classList.toggle('checked');
      } else {
        queryElems(this.elDropdown, `.menu > .item[data-scope="${CSS.escape(scope)}"]`, (el) => el.classList.remove('checked'));
        elItem.classList.toggle('checked', true);
      }
    } else {
      if (this.selectionMode === 'multiple') {
        elItem.classList.toggle('checked');
      } else {
        queryElems(this.elDropdown, `.menu > .item.checked`, (el) => el.classList.remove('checked'));
        elItem.classList.toggle('checked', true);
      }
    }
    this.elComboValue.value = this.collectCheckedValues().join(',');
    this.onChange();
  }

  async onHide() {
    if (this.selectionMode === 'multiple') this.doUpdate();
  }

  init() {
    // init the checked items from initial value
    if (this.elComboValue.value && this.elComboValue.value !== '0' && !queryElems(this.elDropdown, `.menu > .item.checked`).length) {
      const values = this.elComboValue.value.split(',');
      for (const value of values) {
        const elItem = this.elDropdown.querySelector<HTMLElement>(`.menu > .item[data-value="${CSS.escape(value)}"]`);
        elItem?.classList.add('checked');
      }
      this.updateUiList(values);
    }
    this.initialValues = this.collectCheckedValues();

    this.elDropdown.addEventListener('click', (e) => this.onItemClick(e));

    fomanticQuery(this.elDropdown).dropdown('setting', {
      action: 'nothing', // do not hide the menu if user presses Enter
      fullTextSearch: 'exact',
      onHide: () => this.onHide(),
    });
  }
}

export function initIssueSidebarComboList(container: HTMLElement) {
  new IssueSidebarComboList(container).init();
}
