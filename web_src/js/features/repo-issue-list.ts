import {createElementFromAttrs, createElementFromHTML, toggleElem, queryElems} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {confirmModal} from './comp/ConfirmModal.ts';
import {createSortable} from '../modules/sortable.ts';
import {DELETE, GET, POST} from '../modules/fetch.ts';
import {initDropdown} from '../modules/dropdown.ts';
import {performFetchAction} from '../modules/fetch-action.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import type {SortableEvent} from 'sortablejs';

type IssuePoster = {avatar_link: string, full_name: string, username: string};

function initRepoIssueListCheckboxes(issueSelectAll: HTMLInputElement) {
  const container = issueSelectAll.closest('.page-content')!;
  const issueCheckboxes = container.querySelectorAll<HTMLInputElement>('.issue-checkbox');
  const issueFilters = container.querySelector<HTMLElement>('#issue-filters')!;
  const issueActions = container.querySelector<HTMLElement>('#issue-actions')!;

  const syncIssueSelectionState = () => {
    const checkedCheckboxes = Array.from(issueCheckboxes).filter((el) => el.checked);
    const anyChecked = Boolean(checkedCheckboxes.length);
    const allChecked = anyChecked && checkedCheckboxes.length === issueCheckboxes.length;

    issueSelectAll.checked = allChecked;
    issueSelectAll.indeterminate = anyChecked && !allChecked;
    // if any issue is selected, show the action panel, otherwise show the filter panel
    toggleElem(issueFilters, !anyChecked);
    toggleElem(issueActions, anyChecked);
    // there are two panels but only one select-all checkbox, so move the checkbox to the visible panel
    const visiblePanel = anyChecked ? issueActions : issueFilters;
    const toolbarLeft = visiblePanel.querySelector('.issue-list-toolbar-left')!;
    toolbarLeft.prepend(issueSelectAll);
  };

  for (const el of issueCheckboxes) {
    el.addEventListener('change', syncIssueSelectionState);
  }

  issueSelectAll.addEventListener('change', () => {
    for (const el of issueCheckboxes) {
      el.checked = issueSelectAll.checked;
    }
    syncIssueSelectionState();
  });

  queryElems(container, '.issue-action', (el) => el.addEventListener('click',
    async (e: MouseEvent) => {
      e.preventDefault();

      const url = el.getAttribute('data-url')!;
      let action = el.getAttribute('data-action')!;
      const elementId = el.getAttribute('data-element-id')!;
      const issueIDs = Array.from(container.querySelectorAll('.issue-checkbox:checked'), (el) => el.getAttribute('data-issue-id')!).join(',');
      if (!issueIDs) return;

      // for label toggle
      if (action === 'toggle' && e.altKey) {
        action = 'toggle-alt';
      }

      // for delete
      if (action === 'delete') {
        const confirmText = el.getAttribute('data-action-delete-confirm')!;
        if (!await confirmModal({content: confirmText, confirmButtonColor: 'red'})) {
          return;
        }
      }

      const data = new URLSearchParams({action, issue_ids: issueIDs, id: elementId});
      await performFetchAction(el, {method: 'post', url, data});
    },
  ));
}

function initDropdownUserRemoteSearch(el: HTMLElement) {
  const searchUrl = el.getAttribute('data-search-url');
  const actionJumpUrl = el.getAttribute('data-action-jump-url')!;
  const getUserUrl = (username: string) => actionJumpUrl.replace('{username}', () => encodeURIComponent(username));
  const selectedUsername = el.querySelector<HTMLInputElement>('.dropdown-filter')!.value;
  const clearItem = el.querySelector<HTMLElement>('.dropdown-item[data-value=""]')!;
  clearItem.setAttribute('href', getUserUrl(''));
  initDropdown(el, {
    async loadItems(query, signal) {
      const posters: IssuePoster[] = searchUrl ?
        (await (await GET(`${searchUrl}${searchUrl.includes('?') ? '' : '?'}&q=${encodeURIComponent(query)}`, {signal})).json()).results :
        [];
      const items = posters.map((poster) => {
        const item = createElementFromHTML(html`
          <a class="dropdown-item" href="${getUserUrl(poster.username)}">
            <span class="username-display">
              <img class="ui avatar tw-align-middle" src="${poster.avatar_link}" aria-hidden="true" alt width="20" height="20">
              <span>${poster.username}</span>
              ${poster.full_name ? htmlRaw(html`<span class="username-fullname">(${poster.full_name})</span>`) : ''}
            </span>
          </a>
        `);
        item.classList.toggle('active', selectedUsername.toLowerCase() === poster.username.toLowerCase());
        return item;
      });
      if (!query) return [clearItem, ...items];
      if (!items.length) items.push(createElementFromAttrs('a', {class: 'dropdown-item active', href: getUserUrl(query)}, query));
      return [...items, clearItem];
    },
  });
}

function initPinRemoveButton() {
  for (const button of document.querySelectorAll('.issue-card-unpin')) {
    button.addEventListener('click', async (event) => {
      const el = event.currentTarget as HTMLElement;
      const id = Number(el.getAttribute('data-issue-id'));

      // Send the unpin request
      const response = await DELETE(el.getAttribute('data-unpin-url')!);
      if (response.ok) {
        // Delete the tooltip
        el._tippy.destroy();
        // Remove the Card
        el.closest(`div.issue-card[data-issue-id="${CSS.escape(String(id))}"]`)!.remove();
      }
    });
  }
}

async function pinMoveEnd(e: SortableEvent) {
  const url = e.item.getAttribute('data-move-url')!;
  const id = Number(e.item.getAttribute('data-issue-id'));
  await POST(url, {data: {id, position: e.newIndex! + 1}});
}

function initIssuePinSort(pinDiv: HTMLElement) {
  initPinRemoveButton();

  // If only one issue pinned, we don't need to make this Sortable
  if (pinDiv.children.length < 2) return;

  createSortable(pinDiv, {
    group: 'shared',
    onEnd: (e) => {
      (async () => {
        await pinMoveEnd(e);
      })();
    },
  });
}

export function initRepoIssueList() {
  registerGlobalInitFunc('initRepoIssueListCheckboxes', initRepoIssueListCheckboxes);
  registerGlobalInitFunc('initDropdownUserRemoteSearch', initDropdownUserRemoteSearch);
  registerGlobalInitFunc('initIssuePinSort', initIssuePinSort);
}
