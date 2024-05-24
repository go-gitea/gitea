import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {parseDom} from '../utils.js';

// modified from   ./repo-issue-list.js initRepoIssueListAuthorDropdown
function initUserDropdown() {
  const $searchDropdown = $('.user-remote-search');
  if (!$searchDropdown.length) return;

  let searchUrl = $searchDropdown[0].getAttribute('data-search-url');
  const actionJumpUrl = $searchDropdown[0].getAttribute('data-action-jump-url');
  const selectedUserId = $searchDropdown[0].getAttribute('data-selected-user-id');
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
          encodeURIComponent(previousLabels ?? ''),
        );

        for (const item of resp.results) {
          let html = `<a href=${href.replace('{username}', encodeURIComponent(item.username))}><img class="ui avatar tw-align-middle" src="${htmlEscape(item.avatar_link)}" aria-hidden="true" alt="" width="20" height="20"><span class="gt-ellipsis">${htmlEscape(item.username)}</span></a>`;
          if (item.full_name) {
            html += `<span class='search-fullname tw-ml-2'>${htmlEscape(item.full_name)}</span>`;
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
    const menu = $searchDropdown.find('> .menu')[0];
    // remove old dynamic items
    for (const el of menu.querySelectorAll(':scope > .dynamic-item')) {
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
      menu.append(div, ...newMenuItems);
    }
    $searchDropdown.dropdown('refresh');
    // defer our selection to the next tick, because dropdown will set the selection item after this `menu` function
    setTimeout(() => {
      for (const el of menu.querySelectorAll('.item.active, .item.selected')) {
        el.classList.remove('active', 'selected');
      }
      menu.querySelector(`.item[data-value="${selectedUserId}"]`)?.classList.add('selected');
    }, 0);
  };
}

function initShowCard() {
  const urlParams = new URLSearchParams(window.location.search);
  const previousLabels = urlParams.get('labels');
  const previousAssignee = urlParams.get('assignee');
  if (previousLabels || previousAssignee) return;

  const cards = document.querySelectorAll('.issue-card[data-issue]');
  for (const card of cards) {
    card.style.display = 'flex';
  }
}

function initUpdateLabelHref() {
  const url = window.location.href.split('?')[0];
  const urlParams = new URLSearchParams(window.location.search);
  const previousLabels = urlParams.get('labels');
  const previousAssignee = urlParams.get('assignee');
  const labels = document.querySelectorAll(
    '.label-filter > .menu > a.label-filter-item',
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
      label.href = `${url}?${previousAssignee ? `assignee=${previousAssignee}` : ''}&labels=${previousLabels.split(',').filter((l) => l !== `${labelId}`).join(',')}`;
    } else {
      // otherwise add label to href
      const labelsQuery = `&labels=${previousLabels === null || previousLabels.length === 0 ? '' : `${previousLabels},`}`;
      label.href = `${url}?${previousAssignee ? `assignee=${previousAssignee}` : ''}${labelsQuery}${labelId}`;
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

// this function is modified version from https://github.com/go-gitea/gitea/pull/21963
function initProjectCardFilter() {
  const urlParams = new URLSearchParams(window.location.search);
  const labels = urlParams.get('labels');
  const assignee = urlParams.get('assignee');

  if (!labels) {
    if (!assignee) return;
    // loop through all issue cards and check if they are assigned to this user
    const cards = document.querySelectorAll('.issue-card[data-issue]');
    for (const card of cards) {
      const username = card.querySelector('[data-username]');
      if ($(username).data('username') === assignee) {
        card.style.display = 'flex';
      }
    }
    return;
  }

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
    if (!assignee && labelsArray.some((l) => allLables.includes(l))) {
      card.style.display = 'flex';
      continue;
    }
    const username = card.querySelector('[data-username]');
    if ($(username).data('username') === assignee && labelsArray.some((l) => allLables.includes(l))) {
      card.style.display = 'flex';
    }
  }
}

export function initProjectFilter() {
  if (!document.querySelectorAll('.page-content.repository.projects.view-project').length) return;
  initShowCard();
  initProjectCardFilter();
  initUpdateLabelHref();
  initUserDropdown();
}
