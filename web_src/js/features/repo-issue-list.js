import $ from 'jquery';
import {updateIssuesMeta} from './repo-issue.js';
import {toggleElem} from '../utils/dom.js';
import {htmlEscape} from 'escape-goat';

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
    $('#issue-filters, #issue-actions').filter(':visible').find('.column:first').prepend($issueSelectAll);
  };

  $issueCheckboxes.on('change', syncIssueSelectionState);

  $issueSelectAll.on('change', () => {
    $issueCheckboxes.prop('checked', $issueSelectAll.is(':checked'));
    syncIssueSelectionState();
  });

  $('.issue-action').on('click', async function (e) {
    e.preventDefault();
    let action = this.getAttribute('data-action');
    let elementId = this.getAttribute('data-element-id');
    const url = this.getAttribute('data-url');
    const issueIDs = $('.issue-checkbox:checked').map((_, el) => {
      return el.getAttribute('data-issue-id');
    }).get().join(',');
    if (elementId === '0' && url.slice(-9) === '/assignee') {
      elementId = '';
      action = 'clear';
    }
    if (action === 'toggle' && e.altKey) {
      action = 'toggle-alt';
    }
    updateIssuesMeta(
      url,
      action,
      issueIDs,
      elementId
    ).then(() => {
      window.location.reload();
    }).catch((reason) => {
      window.alert(reason.responseJSON.error);
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
      $menu.append('<div class="ui divider dynamic-item"></div>', ...$newMenuItems);
    }
    $searchDropdown.dropdown('refresh');
    // defer our selection to the next tick, because dropdown will set the selection item after this `menu` function
    setTimeout(() => {
      $menu.find('.item.active, .item.selected').removeClass('active selected');
      $menu.find(`.item[data-value="${selectedUserId}"]`).addClass('selected');
    }, 0);
  };
}

export function initRepoIssueList() {
  if (!document.querySelectorAll('.page-content.repository.issue-list, .page-content.repository.milestone-issue-list').length) return;
  initRepoIssueListCheckboxes();
  initRepoIssueListAuthorDropdown();
}
