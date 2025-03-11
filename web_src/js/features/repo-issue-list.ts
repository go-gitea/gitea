import {updateIssuesMeta} from './repo-common.ts';
import {toggleElem, isElemHidden, queryElems} from '../utils/dom.ts';
import {htmlEscape} from 'escape-goat';
import {confirmModal} from './comp/ConfirmModal.ts';
import {showErrorToast} from '../modules/toast.ts';
import {createSortable} from '../modules/sortable.ts';
import {DELETE, POST} from '../modules/fetch.ts';
import {parseDom} from '../utils.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import type {SortableEvent} from 'sortablejs';

function initRepoIssueListCheckboxes() {
  const issueSelectAll = document.querySelector<HTMLInputElement>('.issue-checkbox-all');
  if (!issueSelectAll) return; // logged out state
  const issueCheckboxes = document.querySelectorAll<HTMLInputElement>('.issue-checkbox');

  const syncIssueSelectionState = () => {
    const checkedCheckboxes = Array.from(issueCheckboxes).filter((el) => el.checked);
    const anyChecked = Boolean(checkedCheckboxes.length);
    const allChecked = anyChecked && checkedCheckboxes.length === issueCheckboxes.length;

    if (allChecked) {
      issueSelectAll.checked = true;
      issueSelectAll.indeterminate = false;
    } else if (anyChecked) {
      issueSelectAll.checked = false;
      issueSelectAll.indeterminate = true;
    } else {
      issueSelectAll.checked = false;
      issueSelectAll.indeterminate = false;
    }
    // if any issue is selected, show the action panel, otherwise show the filter panel
    toggleElem('#issue-filters', !anyChecked);
    toggleElem('#issue-actions', anyChecked);
    // there are two panels but only one select-all checkbox, so move the checkbox to the visible panel
    const panels = document.querySelectorAll('#issue-filters, #issue-actions');
    const visiblePanel = Array.from(panels).find((el) => !isElemHidden(el));
    const toolbarLeft = visiblePanel.querySelector('.issue-list-toolbar-left');
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

  queryElems(document, '.issue-action', (el) => el.addEventListener('click',
    async (e: MouseEvent) => {
      e.preventDefault();

      const url = el.getAttribute('data-url');
      let action = el.getAttribute('data-action');
      let elementId = el.getAttribute('data-element-id');
      const issueIDList: string[] = [];
      for (const el of document.querySelectorAll('.issue-checkbox:checked')) {
        issueIDList.push(el.getAttribute('data-issue-id'));
      }
      const issueIDs = issueIDList.join(',');
      if (!issueIDs) return;

      // for assignee
      if (elementId === '0' && url.endsWith('/assignee')) {
        elementId = '';
        action = 'clear';
      }

      // for toggle
      if (action === 'toggle' && e.altKey) {
        action = 'toggle-alt';
      }

      // for delete
      if (action === 'delete') {
        const confirmText = el.getAttribute('data-action-delete-confirm');
        if (!await confirmModal({content: confirmText, confirmButtonColor: 'red'})) {
          return;
        }
      }

      try {
        await updateIssuesMeta(url, action, issueIDs, elementId);
        window.location.reload();
      } catch (err) {
        showErrorToast(err.responseJSON?.error ?? err.message);
      }
    },
  ));
}

function initDropdownUserRemoteSearch(el: Element) {
  let searchUrl = el.getAttribute('data-search-url');
  const actionJumpUrl = el.getAttribute('data-action-jump-url');
  let selectedUsername = el.getAttribute('data-selected-username') || '';
  const $searchDropdown = fomanticQuery(el);
  const elMenu = el.querySelector('.menu');
  const elSearchInput = el.querySelector<HTMLInputElement>('.ui.search input');
  const elItemFromInput = el.querySelector('.menu > .item-from-input');

  $searchDropdown.dropdown('setting', {
    fullTextSearch: true,
    selectOnKeydown: false,
    action: (_text: string, value: string) => {
      window.location.href = actionJumpUrl.replace('{username}', encodeURIComponent(value));
    },
  });

  const selectUsername = (username: string) => {
    queryElems(elMenu, '.item.active, .item.selected', (el) => el.classList.remove('active', 'selected'));
    elMenu.querySelector(`.item[data-value="${CSS.escape(username)}"]`)?.classList.add('selected');
  };

  type ProcessedResult = {value: string, name: string};
  const processedResults: ProcessedResult[] = []; // to be used by dropdown to generate menu items
  const syncItemFromInput = () => {
    const inputVal = elSearchInput.value.trim();
    elItemFromInput.setAttribute('data-value', inputVal);
    elItemFromInput.textContent = inputVal;
    const showItemFromInput = !processedResults.length && inputVal !== '';
    toggleElem(elItemFromInput, showItemFromInput);
    selectUsername(showItemFromInput ? inputVal : selectedUsername);
  };

  elSearchInput.value = selectedUsername;
  if (!searchUrl) {
    elSearchInput.addEventListener('input', syncItemFromInput);
  } else {
    if (!searchUrl.includes('?')) searchUrl += '?';
    $searchDropdown.dropdown('setting', 'apiSettings', {
      cache: false,
      url: `${searchUrl}&q={query}`,
      onResponse(resp: any) {
        // the content is provided by backend IssuePosters handler
        processedResults.length = 0;
        for (const item of resp.results) {
          let html = `<img class="ui avatar tw-align-middle" src="${htmlEscape(item.avatar_link)}" aria-hidden="true" alt="" width="20" height="20"><span class="gt-ellipsis">${htmlEscape(item.username)}</span>`;
          if (item.full_name) html += `<span class="search-fullname tw-ml-2">${htmlEscape(item.full_name)}</span>`;
          if (selectedUsername.toLowerCase() === item.username.toLowerCase()) selectedUsername = item.username;
          processedResults.push({value: item.username, name: html});
        }
        resp.results = processedResults;
        return resp;
      },
    });
    $searchDropdown.dropdown('setting', 'onShow', () => $searchDropdown.dropdown('filter', ' ')); // trigger a search on first show
  }

  // we want to generate the dropdown menu items by ourselves, replace its internal setup functions
  const dropdownSetup = {...$searchDropdown.dropdown('internal', 'setup')};
  const dropdownTemplates = $searchDropdown.dropdown('setting', 'templates');
  $searchDropdown.dropdown('internal', 'setup', dropdownSetup);
  dropdownSetup.menu = function (values: any) {
    // remove old dynamic items
    for (const el of elMenu.querySelectorAll(':scope > .dynamic-item')) {
      el.remove();
    }

    const newMenuHtml = dropdownTemplates.menu(values, $searchDropdown.dropdown('setting', 'fields'), true /* html */, $searchDropdown.dropdown('setting', 'className'));
    if (newMenuHtml) {
      const newMenuItems = parseDom(newMenuHtml, 'text/html').querySelectorAll('body > div');
      for (const newMenuItem of newMenuItems) {
        newMenuItem.classList.add('dynamic-item');
      }
      const div = document.createElement('div');
      div.classList.add('divider', 'dynamic-item');
      elMenu.append(div, ...newMenuItems);
    }
    $searchDropdown.dropdown('refresh');
    // defer our selection to the next tick, because dropdown will set the selection item after this `menu` function
    setTimeout(() => syncItemFromInput(), 0);
  };
}

function initPinRemoveButton() {
  for (const button of document.querySelectorAll('.issue-card-unpin')) {
    button.addEventListener('click', async (event) => {
      const el = event.currentTarget as HTMLElement;
      const id = Number(el.getAttribute('data-issue-id'));

      // Send the unpin request
      const response = await DELETE(el.getAttribute('data-unpin-url'));
      if (response.ok) {
        // Delete the tooltip
        el._tippy.destroy();
        // Remove the Card
        el.closest(`div.issue-card[data-issue-id="${id}"]`).remove();
      }
    });
  }
}

async function pinMoveEnd(e: SortableEvent) {
  const url = e.item.getAttribute('data-move-url');
  const id = Number(e.item.getAttribute('data-issue-id'));
  await POST(url, {data: {id, position: e.newIndex + 1}});
}

async function initIssuePinSort() {
  const pinDiv = document.querySelector('#issue-pins');

  if (pinDiv === null) return;

  // If the User is not a Repo Admin, we don't need to proceed
  if (!pinDiv.hasAttribute('data-is-repo-admin')) return;

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
  if (document.querySelector('.page-content.repository.issue-list, .page-content.repository.milestone-issue-list')) {
    initRepoIssueListCheckboxes();
    queryElems(document, '.ui.dropdown.user-remote-search', (el) => initDropdownUserRemoteSearch(el));
    initIssuePinSort();
  } else if (document.querySelector('.page-content.dashboard.issues')) {
    // user or org home: issue list, pull request list
    queryElems(document, '.ui.dropdown.user-remote-search', (el) => initDropdownUserRemoteSearch(el));
  }
}
