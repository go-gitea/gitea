import {fomanticQuery} from '../modules/fomantic/base.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {addDelegatedEventListener, queryElemChildren, queryElems, toggleElem} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {parseDom} from '../utils.ts';

export function syncIssueMainContentTimelineItems(oldMainContent: Element, newMainContent: Element) {
  // find the end of comments timeline by "id=timeline-comments-end" in current main content, and insert new items before it
  const timelineEnd = oldMainContent.querySelector('.timeline-item[id="timeline-comments-end"]');
  if (!timelineEnd) return;

  const oldTimelineItems = oldMainContent.querySelectorAll(`.timeline-item[id]`);
  for (const oldItem of oldTimelineItems) {
    const oldItemId = oldItem.getAttribute('id')!;
    const newItem = newMainContent.querySelector(`.timeline-item[id="${CSS.escape(oldItemId)}"]`);
    if (oldItem.classList.contains('event') && !newItem) {
      // if the item is not in new content, we want to remove it from old content only if it's an event item, otherwise we keep it
      oldItem.remove();
    }
  }

  const newTimelineItems = newMainContent.querySelectorAll(`.timeline-item[id]`);
  for (const newItem of newTimelineItems) {
    const newItemId = newItem.getAttribute('id')!;
    const oldItem = oldMainContent.querySelector(`.timeline-item[id="${CSS.escape(newItemId)}"]`);
    if (oldItem) {
      if (oldItem.classList.contains('event')) {
        // for event item (e.g.: "add & remove labels"), we want to replace the existing one if exists
        // because the label operations can be merged into one event item, so the new item might be different from the old one
        oldItem.replaceWith(newItem);
      }
      continue;
    }
    timelineEnd.insertAdjacentElement('beforebegin', newItem);
  }
}

export class IssueSidebarComboList {
  updateUrl: string;
  updateAlgo: string;
  selectionMode: string;
  elDropdown: HTMLElement;
  elList: HTMLElement | null;
  elComboValue: HTMLInputElement;
  initialValues: string[] = [];
  container: HTMLElement;

  elIssueMainContent: HTMLElement;
  elIssueSidebar: HTMLElement;

  constructor(container: HTMLElement) {
    this.container = container;
    this.updateUrl = container.getAttribute('data-update-url')!;
    this.updateAlgo = container.getAttribute('data-update-algo')!;
    this.selectionMode = container.getAttribute('data-selection-mode')!;
    if (!['single', 'multiple'].includes(this.selectionMode)) throw new Error(`Invalid data-update-on: ${this.selectionMode}`);
    if (!['diff', 'all'].includes(this.updateAlgo)) throw new Error(`Invalid data-update-algo: ${this.updateAlgo}`);
    this.elDropdown = container.querySelector<HTMLElement>(':scope > .ui.dropdown')!;
    this.elList = container.querySelector<HTMLElement>(':scope > .ui.list');
    this.elComboValue = container.querySelector<HTMLInputElement>(':scope > .combo-value')!;

    this.elIssueMainContent = document.querySelector('.issue-content-left')!;
    this.elIssueSidebar = document.querySelector('.issue-content-right')!;
  }

  collectCheckedValues() {
    return Array.from(this.elDropdown.querySelectorAll('.menu > .item.checked'), (el) => el.getAttribute('data-value')!);
  }

  updateUiList(changedValues: Array<string>) {
    if (!this.elList) return;
    const elEmptyTip = this.elList.querySelector(':scope > .item.empty-list')!;
    queryElemChildren(this.elList, '.item:not(.empty-list)', (el) => el.remove());

    const isCreatePageProjectCombo = !this.updateUrl && this.container.classList.contains('sidebar-project-combo');

    for (const value of changedValues) {
      if (isCreatePageProjectCombo) {
        const tpl = document.querySelector<HTMLElement>(`.js-project-card-template[data-project-id="${CSS.escape(value)}"]`);
        if (!tpl) continue;
        const card = tpl.firstElementChild?.cloneNode(true) as HTMLElement | undefined;
        if (!card) continue;
        this.elList.append(card);
        for (const sub of card.querySelectorAll<HTMLElement>('.issue-sidebar-combo')) {
          new IssueSidebarComboList(sub).init();
        }
        continue;
      }
      const el = this.elDropdown.querySelector<HTMLElement>(`.menu > .item[data-value="${CSS.escape(value)}"]`);
      if (!el) continue;
      const listItem = el.cloneNode(true) as HTMLElement;
      queryElems(listItem, '.item-check-mark, .item-secondary-info', (el) => el.remove());
      this.elList.append(listItem);
    }
    const hasItems = Boolean(this.elList.querySelector('.item:not(.empty-list)'));
    toggleElem(elEmptyTip, !hasItems);
    if (isCreatePageProjectCombo) this.recomputeProjectBoardCarrier();
  }

  async reloadPagePartially() {
    const resp = await GET(window.location.href);
    if (!resp.ok) throw new Error(`Failed to reload page: ${resp.statusText}`);
    const doc = parseDom(await resp.text(), 'text/html');

    // we can safely replace the whole right part (sidebar) because there are only some dropdowns and lists
    const newSidebar = doc.querySelector('.issue-content-right')!;
    this.elIssueSidebar.replaceWith(newSidebar);

    // for the main content (left side), at the moment we only support handling known timeline items
    const newMainContent = doc.querySelector('.issue-content-left')!;
    syncIssueMainContentTimelineItems(this.elIssueMainContent, newMainContent);
  }

  async sendRequestToBackend(changedValues: Array<string>): Promise<Response | null> {
    let lastResp: Response | null = null;
    if (this.updateAlgo === 'diff') {
      for (const value of this.initialValues) {
        if (!changedValues.includes(value)) {
          lastResp = await POST(this.updateUrl, {data: new URLSearchParams({action: 'detach', id: value})});
          if (!lastResp.ok) return lastResp;
        }
      }
      for (const value of changedValues) {
        if (!this.initialValues.includes(value)) {
          lastResp = await POST(this.updateUrl, {data: new URLSearchParams({action: 'attach', id: value})});
          if (!lastResp.ok) return lastResp;
        }
      }
    } else {
      lastResp = await POST(this.updateUrl, {data: new URLSearchParams({id: changedValues.join(',')})});
    }
    return lastResp;
  }

  async updateToBackend(changedValues: Array<string>) {
    this.elIssueSidebar.classList.add('is-loading');
    try {
      const resp = await this.sendRequestToBackend(changedValues);
      if (!resp) return; // no request sent, no need to reload
      if (!resp.ok) {
        showErrorToast(`Failed to update to backend: ${resp.statusText}`);
        return;
      }
      await this.reloadPagePartially();
    } catch (e) {
      console.error('Failed to update to backend', e);
      showErrorToast(`Failed to update to backend: ${errorMessage(e)}`);
    } finally {
      this.elIssueSidebar.classList.remove('is-loading');
    }
  }

  // On the create page there is no POST/reload, so the column combo's visible
  // label must be synced client-side from the checked menu item (the edit page
  // gets this for free via reloadPagePartially).
  syncDeferredColumnLabel() {
    const elLabel = this.elDropdown.querySelector<HTMLElement>('.fixed-text');
    if (!elLabel) return;
    const elChecked = this.elDropdown.querySelector<HTMLElement>('.menu > .item.checked');
    const elTriangle = elLabel.querySelector('svg');
    queryElemChildren(elLabel, '.color-icon, .gt-ellipsis', (el) => el.remove());
    const elText = document.createElement('div');
    elText.classList.add('gt-ellipsis');
    if (elChecked) {
      const elColor = queryElemChildren(elChecked, '.color-icon')[0];
      if (elColor) elLabel.insertBefore(elColor.cloneNode(true), elTriangle);
      elText.textContent = elChecked.querySelector('.gt-ellipsis')?.textContent ?? '';
    } else {
      elText.textContent = elLabel.getAttribute('data-no-column-text') ?? '';
    }
    elLabel.insertBefore(elText, elTriangle);
  }

  recomputeProjectBoardCarrier() {
    const carrier = document.querySelector<HTMLInputElement>('.js-project-board-ids');
    if (!carrier) return; // not the create page
    const pairs: string[] = [];
    for (const combo of document.querySelectorAll<HTMLElement>('.sidebar-project-column-combo')) {
      // skip the hidden card-clone source; only the visible selected cards count
      if (combo.closest('.js-project-card-templates')) continue;
      const projectId = combo.getAttribute('data-project-id');
      if (!projectId) continue;
      const elComboValue = queryElemChildren<HTMLInputElement>(combo, '.combo-value')[0];
      const val = elComboValue?.value || '';
      if (val) pairs.push(`${projectId}:${val}`);
    }
    carrier.value = pairs.join(',');
  }

  async doUpdate() {
    const changedValues = this.collectCheckedValues();
    if (this.initialValues.join(',') === changedValues.join(',')) return;
    if (!this.updateUrl) this.updateUiList(changedValues);
    if (this.updateUrl) await this.updateToBackend(changedValues);
    if (!this.updateUrl && this.container.classList.contains('sidebar-project-column-combo')) {
      this.syncDeferredColumnLabel();
      this.recomputeProjectBoardCarrier();
    }
    this.initialValues = changedValues;
  }

  async onChange() {
    if (this.selectionMode === 'single') {
      await this.doUpdate();
      fomanticQuery(this.elDropdown).dropdown('hide');
    }
  }

  async onItemClick(elItem: HTMLElement, e: Event) {
    e.preventDefault();
    if (elItem.hasAttribute('data-can-change') && elItem.getAttribute('data-can-change') !== 'true') return;

    if (elItem.matches('.clear-selection')) {
      queryElems(this.elDropdown, '.menu > .item', (el) => el.classList.remove('checked'));
      this.elComboValue.value = '';
      // onChange only acts for single-select; on the create page (deferred,
      // no updateUrl) the multi-select project combo needs doUpdate() to drop
      // the now-unselected cards and refresh the carrier, otherwise a cleared
      // default project keeps its stale card and still gets assigned. The edit
      // path (updateUrl set) keeps its existing onChange()/onHide() behavior.
      if (this.selectionMode !== 'single' && !this.updateUrl) {
        this.doUpdate();
      } else {
        this.onChange();
      }
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
    // On the create page (deferred, no updateUrl) the multi-select project
    // combo must sync the rendered cards and the form carrier on every toggle:
    // onChange() is a no-op for multi-select and onHide() only fires when the
    // dropdown closes, so the form could be submitted with a stale carrier
    // (or an orphaned card for a just-deselected project). The edit path
    // (updateUrl set) keeps its existing onChange()/onHide() behavior.
    if (this.selectionMode !== 'single' && !this.updateUrl) {
      this.doUpdate();
    } else {
      this.onChange();
    }
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
      if (this.elList && this.elList.getAttribute('data-combo-list-inited') !== 'true') {
        this.updateUiList(values);
      }
    }
    this.initialValues = this.collectCheckedValues();

    addDelegatedEventListener(this.elDropdown, 'click', '.item', (el, e) => this.onItemClick(el, e));

    fomanticQuery(this.elDropdown).dropdown('setting', {
      action: 'nothing', // do not hide the menu if user presses Enter
      fullTextSearch: 'exact',
      hideDividers: 'empty',
      onHide: () => this.onHide(),
    });
  }
}
