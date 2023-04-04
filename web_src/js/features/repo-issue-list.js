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
    });
  });
}

function initRepoIssueListAuthorDropdown() {
  const $searchDropdown = $('.user-remote-search');
  if (!$searchDropdown.length) return;

  // TODO: the data-selected-user-id is not used yet, it seems unnecessary
  let searchUrl = $searchDropdown.attr('data-search-url');
  let actionJumpUrl = $searchDropdown.attr('data-action-jump-url');
  if (searchUrl.indexOf('?') === -1) searchUrl += '?';

  $searchDropdown.dropdown('setting', {
    fullTextSearch: true,
    selectOnKeydown: false,
    apiSettings: {
      cache: false,
      url: `${searchUrl}&q={query}`,
      onResponse(resp) {
        // the content is provided by backend IssuePosters handler
        for (const item of resp.results) {
          item.value = item.user_id;
          item.name = `<img class="ui avatar gt-vm gt-mr-2" src="${htmlEscape(item.avatar_link)}">${htmlEscape(item.username)}`;
          if (item.full_name) {
            item.name += `(${htmlEscape(item.full_name)})`;
          }
        }
        return resp;
      },
    },
    action: (_text, value) => {
      window.location.href = actionJumpUrl.replace('{user_id}', encodeURIComponent(value));
    },
  });

  const dropdownSetup = {...$searchDropdown.dropdown('internal', 'setup')};
  const dropdownSetting = (...args) => $searchDropdown.dropdown('setting', ...args);
  const dropdownTemplates = dropdownSetting('templates');
  $searchDropdown.dropdown('internal', 'setup', dropdownSetup);
  dropdownSetup.menu = function (values) {
    // we want to generate the dropdown menu items by ourselves
    const $menu = $searchDropdown.find('> .menu');
    const menusHtml = dropdownTemplates.menu(values, dropdownSetting('fields'), true /* html */, dropdownSetting('className'));
    const $items = $menu.find('> .ui.divider, > .item');

    // replace the menu items after the divider
    let elDivider;
    for (const el of $items) {
      if (!elDivider && el.classList.contains('divider')) {
        elDivider = el;
      } else if (elDivider) {
        el.remove();
      }
    }
    elDivider?.remove();
    if (menusHtml) {
      $menu.append('<div class="ui divider"></div>');
      $menu.append(...$(menusHtml));
    }
    $searchDropdown.dropdown('refresh');
  }
}

export function initRepoIssueList() {
  if (!document.querySelectorAll('.page-content.repository.issue-list, .page-content.repository.milestone-issue-list').length) return;
  initRepoIssueListCheckboxes();
  initRepoIssueListAuthorDropdown();
}
