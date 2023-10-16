import $ from 'jquery';
import {htmlEscape} from 'escape-goat';

// modified from   ./repo-issue-list.js initRepoIssueListAuthorDropdown
function initUserDropdown() {
  const $searchDropdown = $('#assigneeDropdown.user-remote-search');
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
        const urlParams = new URLSearchParams(window.location.search);
        const previousLabels = urlParams.get('labels');
        const href = actionJumpUrl.replace(
          '{labels}',
          encodeURIComponent(previousLabels ?? '')
        );

        for (const item of resp.results) {
          let html = `<a href=${href.replace('{username}', encodeURIComponent(item.username))}><img class='ui avatar gt-vm' src='${htmlEscape(item.avatar_link)}' aria-hidden='true' alt='' width='20' height='20'><span class='gt-ellipsis gt-px-4' >${htmlEscape(item.username)}</span></a>`;
          if (item.full_name) {
            html += `<span class='search-fullname gt-ml-3'>${htmlEscape(item.full_name)}</span>`;
          }
          processedResults.push({value: item.username, name: html});
        }
        resp.results = processedResults;
        return resp;
      },
    },
    action: (_text, _value) => {},
    onShow: () => {
      $searchDropdown.dropdown('filter', ' '); // trigger a search on first show

      const urlParams = new URLSearchParams(window.location.search);
      const previousLabels = urlParams.get('labels');
      const labelsQuery = `&labels=${previousLabels === null || previousLabels.length === 0 ? '' : `${previousLabels},`}`;

      const noAssignee = document.getElementById('no-assignee');
      noAssignee.href = `${window.location.href.split('?')[0]}?assignee=${labelsQuery}`;
    },
  });

  // we want to generate the dropdown menu items by ourselves, replace its internal setup functions
  const dropdownSetup = {...$searchDropdown.dropdown('internal', 'setup')};
  const dropdownTemplates = $searchDropdown.dropdown('setting', 'templates');
  $searchDropdown.dropdown('internal', 'setup', dropdownSetup);
  dropdownSetup.menu = function (values) {
    const $menu = $searchDropdown.find('> .menu');
    $menu.find('> .dynamic-item').remove(); // remove old dynamic items

    const newMenuHtml = dropdownTemplates.menu(
      values,
      $searchDropdown.dropdown('setting', 'fields'),
      true /* html */,
      $searchDropdown.dropdown('setting', 'className')
    );
    if (newMenuHtml) {
      const $newMenuItems = $(newMenuHtml);
      $newMenuItems.addClass('dynamic-item');
      $menu.append(
        `<div class='divider dynamic-item'></div>`,
        ...$newMenuItems
      );
    }
    $searchDropdown.dropdown('refresh');
    // defer our selection to the next tick, because dropdown will set the selection item after this `menu` function
    setTimeout(() => {
      $menu.find('.item.active, .item.selected').removeClass('active selected');
      $menu.find(`.item[data-value='${selectedUserId}']`).addClass('selected');
    }, 0);
  };
}

function initProjectFilterHref() {
  if (!window.location.href.includes('?assignee=')) {
    window.location.href += '?assignee=';
  }
}

function initUpdateLabelHref() {
  const urlParams = new URLSearchParams(window.location.search);
  const previousLabels = urlParams.get('labels');
  const labels = document.querySelectorAll(
    '.label-filter > .menu > a.label-filter-item'
  );

  for (const label of labels) {
    const labelId = $(label).data('label-id');

    if (typeof labelId !== 'number') continue;
    // if label is already selected, remove label from href
    if (
      previousLabels &&
      previousLabels.length > 0 &&
      previousLabels.split(',').includes(`${labelId}`)
    ) {
      label.href = `${window.location.href.split('&labels')[0]}'&labels='${previousLabels.split(',').filter((l) => l !== `${labelId}`).join(',')}`;
    } else {
      // otherwise add label to href
      const labelsQuery = `&labels=${previousLabels === null || previousLabels.length === 0 ? '' : `${previousLabels},`}`;
      label.href = window.location.href.split('&labels')[0] + labelsQuery + labelId;
    }

    // only show checkmark for selected labels
    if (
      !previousLabels ||
      previousLabels.length === 0 ||
      !previousLabels.split(',').includes(labelId.toString())
    ) {
      const checkMark = label.getElementsByTagName('span');
      if (!checkMark || checkMark.length === 0) continue;
      checkMark[0].style.display = 'none';
    }
  }
}

function initProjectAssigneeFilter() {
  // check if assignee query string is set
  const urlParams = new URLSearchParams(window.location.search);
  const assignee = urlParams.get('assignee');
  if (!assignee) return;

  // loop through all issue cards and check if they are assigned to this user
  const cards = document.querySelectorAll('.issue-card[data-issue]');
  for (const card of cards) {
    const username = card.querySelector('[data-username]');
    if ($(username).data('username') !== assignee) {
      card.style.display = 'none';
    }
  }
}

// this function is modified version from https://github.com/go-gitea/gitea/pull/21963
function initProjectLabelFilter() {
  // FIXME: Per design document, this should be moved to filter server side once sorting is partial ajax send
  //        There is a risk of flash of unfiltered content with this approach

  // check if labels query string is set
  const urlParams = new URLSearchParams(window.location.search);
  const labels = urlParams.get('labels');
  if (!labels) return;

  // split labels query string into array
  const labelsArray = labels.split(',');

  // loop through all cards and check if they have the label
  const cards = document.querySelectorAll('.issue-card[data-issue]');
  for (const card of cards) {
    const labels = card.querySelectorAll('[data-label-id]');
    const allLables = [];
    for (const label of labels) {
      const label_id = $(label).data('label-id');
      if (typeof label_id !== 'number') continue;
      allLables.push(label_id.toString());
    }
    if (!labelsArray.every((l) => allLables.includes(l))) {
      card.style.display = 'none';
    }
  }
}

export function initProjectFilter() {
  if (!document.querySelectorAll('.page-content.repository.projects.view-project').length) return;
  initProjectFilterHref();
  initUpdateLabelHref();
  initProjectAssigneeFilter();
  initProjectLabelFilter();
  initUserDropdown();
}
