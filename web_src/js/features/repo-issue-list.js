import $ from 'jquery';
import {updateIssuesMeta} from './repo-issue.js';
import {toggleElem, hideElem} from '../utils/dom.js';
import {htmlEscape} from 'escape-goat';
import {confirmModal} from './comp/ConfirmModal.js';
import {showErrorToast} from '../modules/toast.js';
import {createSortable} from '../modules/sortable.js';
import {DELETE, POST} from '../modules/fetch.js';

function initRepoIssueListCheckboxes() {
  const $issueSelectAll = $('.issue-checkbox-all');
  const $issueCheckboxes = $('.issue-checkbox');

  const syncIssueSelectionState = () => {
    const $checked = $issueCheckboxes.filter(':checked');
    const anyChecked = $checked.length !== 0;
    const allChecked = anyChecked && $checked.length === $issueCheckboxes.length;

    if (allChecked) {
      $issueSelectAll.prop({'checked': true, 'indeterminate': false});
    } else if (anyChecked) {
      $issueSelectAll.prop({'checked': false, 'indeterminate': true});
    } else {
      $issueSelectAll.prop({'checked': false, 'indeterminate': false});
    }
    // if any issue is selected, show the action panel, otherwise show the filter panel
    toggleElem($('#issue-filters'), !anyChecked);
    toggleElem($('#issue-actions'), anyChecked);
    // there are two panels but only one select-all checkbox, so move the checkbox to the visible panel
    $('#issue-filters, #issue-actions').filter(':visible').find('.issue-list-toolbar-left').prepend($issueSelectAll);
  };

  $issueCheckboxes.on('change', syncIssueSelectionState);

  $issueSelectAll.on('change', () => {
    $issueCheckboxes.prop('checked', $issueSelectAll.is(':checked'));
    syncIssueSelectionState();
  });

  $('.issue-action').on('click', async function (e) {
    e.preventDefault();

    const url = this.getAttribute('data-url');
    let action = this.getAttribute('data-action');
    let elementId = this.getAttribute('data-element-id');
    let issueIDs = [];
    for (const el of document.querySelectorAll('.issue-checkbox:checked')) {
      issueIDs.push(el.getAttribute('data-issue-id'));
    }
    issueIDs = issueIDs.join(',');
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
      const confirmText = e.target.getAttribute('data-action-delete-confirm');
      if (!await confirmModal({content: confirmText, buttonColor: 'orange'})) {
        return;
      }
    }

    updateIssuesMeta(
      url,
      action,
      issueIDs,
      elementId,
    ).then(() => {
      window.location.reload();
    }).catch((reason) => {
      showErrorToast(reason.responseJSON.error);
    });
  });
}

function initRepoIssueListAuthorDropdown() {
  const $searchDropdown = $('.user-remote-search');
  if (!$searchDropdown.length) return;

  let searchUrl = $searchDropdown.attr('data-search-url');
  const actionJumpUrl = $searchDropdown.attr('data-action-jump-url');
  const selectedUserId = $searchDropdown.attr('data-selected-user-id');
  if (!searchUrl.includes('?')) searchUrl += '?';

  $searchDropdown.dropdown('setting', {
    fullTextSearch: true,
    selectOnKeydown: false,
    apiSettings: {
      cache: false,
      url: `${searchUrl}&q={query}`,
      onResponse(resp) {
        // the content is provided by backend IssuePosters handler
        const processedResults = []; // to be used by dropdown to generate menu items
        for (const item of resp.results) {
          let html = `<img class="ui avatar gt-vm" src="${htmlEscape(item.avatar_link)}" aria-hidden="true" alt="" width="20" height="20"><span class="gt-ellipsis">${htmlEscape(item.username)}</span>`;
          if (item.full_name) html += `<span class="search-fullname gt-ml-3">${htmlEscape(item.full_name)}</span>`;
          processedResults.push({value: item.user_id, name: html});
        }
        resp.results = processedResults;
        return resp;
      },
    },
    action: (_text, value) => {
      window.location.href = actionJumpUrl.replace('{user_id}', encodeURIComponent(value));
    },
    onShow: () => {
      $searchDropdown.dropdown('filter', ' '); // trigger a search on first show
    },
  });

  // we want to generate the dropdown menu items by ourselves, replace its internal setup functions
  const dropdownSetup = {...$searchDropdown.dropdown('internal', 'setup')};
  const dropdownTemplates = $searchDropdown.dropdown('setting', 'templates');
  $searchDropdown.dropdown('internal', 'setup', dropdownSetup);
  dropdownSetup.menu = function (values) {
    const $menu = $searchDropdown.find('> .menu');
    $menu.find('> .dynamic-item').remove(); // remove old dynamic items

    const newMenuHtml = dropdownTemplates.menu(values, $searchDropdown.dropdown('setting', 'fields'), true /* html */, $searchDropdown.dropdown('setting', 'className'));
    if (newMenuHtml) {
      const $newMenuItems = $(newMenuHtml);
      $newMenuItems.addClass('dynamic-item');
      $menu.append('<div class="divider dynamic-item"></div>', ...$newMenuItems);
    }
    $searchDropdown.dropdown('refresh');
    // defer our selection to the next tick, because dropdown will set the selection item after this `menu` function
    setTimeout(() => {
      $menu.find('.item.active, .item.selected').removeClass('active selected');
      $menu.find(`.item[data-value="${selectedUserId}"]`).addClass('selected');
    }, 0);
  };
}

function initPinRemoveButton() {
  for (const button of document.getElementsByClassName('issue-card-unpin')) {
    button.addEventListener('click', async (event) => {
      const el = event.currentTarget;
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

async function pinMoveEnd(e) {
  const url = e.item.getAttribute('data-move-url');
  const id = Number(e.item.getAttribute('data-issue-id'));
  await POST(url, {data: {id, position: e.newIndex + 1}});
}

async function initIssuePinSort() {
  const pinDiv = document.getElementById('issue-pins');

  if (pinDiv === null) return;

  // If the User is not a Repo Admin, we don't need to proceed
  if (!pinDiv.hasAttribute('data-is-repo-admin')) return;

  initPinRemoveButton();

  // If only one issue pinned, we don't need to make this Sortable
  if (pinDiv.children.length < 2) return;

  createSortable(pinDiv, {
    group: 'shared',
    animation: 150,
    ghostClass: 'card-ghost',
    onEnd: pinMoveEnd,
  });
}

function initArchivedLabelFilter() {
  const archivedLabelEl = document.querySelector('#archived-filter-checkbox');
  if (!archivedLabelEl) {
    return;
  }

  const url = new URL(window.location.href);
  const archivedLabels = document.querySelectorAll('[data-is-archived]');

  if (!archivedLabels.length) {
    hideElem('.archived-label-filter');
    return;
  }
  const selectedLabels = (url.searchParams.get('labels') || '')
    .split(',')
    .map((id) => id < 0 ? `${~id + 1}` : id); // selectedLabels contains -ve ids, which are excluded so convert any -ve value id to +ve

  const archivedElToggle = () => {
    for (const label of archivedLabels) {
      const id = label.getAttribute('data-label-id');
      toggleElem(label, archivedLabelEl.checked || selectedLabels.includes(id));
    }
  };

  archivedElToggle();
  archivedLabelEl.addEventListener('change', () => {
    archivedElToggle();
    if (archivedLabelEl.checked) {
      url.searchParams.set('archived', 'true');
    } else {
      url.searchParams.delete('archived');
    }
    window.location.href = url.href;
  });
}

export function initRepoIssueList() {
  if (!document.querySelectorAll('.page-content.repository.issue-list, .page-content.repository.milestone-issue-list').length) return;
  initRepoIssueListCheckboxes();
  initRepoIssueListAuthorDropdown();
  initIssuePinSort();
  initArchivedLabelFilter();
}
