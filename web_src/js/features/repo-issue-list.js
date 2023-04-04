import $ from 'jquery';
import {updateIssuesMeta} from './repo-issue.js';
import {toggleElem} from '../utils/dom.js';

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

  const $authorSearchDropdown = $('.author-search');
  if (!$authorSearchDropdown.length) {
    return;
  }
  $('#author-search-input').on('input', (e) => {
    e.stopImmediatePropagation();
    fetchPostersData($authorSearchDropdown, false);
  });
  // show all results when clicking on the dropdown
  $authorSearchDropdown.on('click', () => {
    // if dropdown is from visible to not, do not need to fetch data
    if ($authorSearchDropdown.attr('aria-expanded') === 'true') {
      return;
    }
    // reset input value
    $('#author-search-input').val('');
    fetchPostersData($authorSearchDropdown, true);
  });

// isShowAll decides if fetching all data or fetching data with search query from user input
  async function fetchPostersData($authorSearchDropdown, isShowAll) {
    const baseUrl = $authorSearchDropdown.attr('data-url');
    const url = isShowAll ? baseUrl : `${baseUrl}?q=${$('#author-search-input').val()}`;
    const res = await fetch(url);
    const postersJson = await res.json();
    if (!postersJson) {
      $authorSearchDropdown.addClass('disabled');
      return;
    }
    // get data needed from data- attributes for generating the poster options
    const posterID = $authorSearchDropdown.attr('data-poster-id');
    const isShowFullName = $authorSearchDropdown.attr('data-show-fullname');
    const posterGeneralUrl = $authorSearchDropdown.attr('data-general-poster-url');
    const $defaultMenu = $authorSearchDropdown.find('.menu');
    // remove former options, then append newly searched posters
    $defaultMenu.find('.item:gt(0)').remove();
    for (let i = 0; i < postersJson.length; i++) {
      const {id, avatar_url, username, full_name} = postersJson[i];
      $defaultMenu.append(`<a class="item gt-df${posterID === id ? ' active selected' : ''}" href="${posterGeneralUrl}${id}">
      <img class="ui avatar gt-vm" src="${avatar_url}" title="${username}" width="28" height="28">
      <span class="gt-ellipsis">${username}${isShowFullName === 'true' ? `<span class="search-fullname"> ${full_name}</span>` : ''}</span>
    </a>`);
    }
    // append aria related attributes to newly added menu items
    const $items = $defaultMenu.find('> .item');
    $items.each((_, item) => updateMenuItem($authorSearchDropdown[0], item));
    $authorSearchDropdown[0][ariaPatchKey].deferredRefreshAriaActiveItem();
  }
}

export function initRepoIssueList() {
  if (!document.querySelectorAll('.page-content.repository.issue-list, .page-content.repository.milestone-issue-list').length) return;
  initRepoIssueListCheckboxes();
  initRepoIssueListAuthorDropdown();
}
