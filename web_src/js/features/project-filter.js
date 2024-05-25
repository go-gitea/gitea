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
        const previousAssignees = urlParams.get('assignees');
        const href = actionJumpUrl.replace(
          '{labels}',
          encodeURIComponent(previousLabels ?? ''),
        );
        for (const item of resp.results) {
          let usernameHref = '';
          if (
            previousAssignees &&
            previousAssignees.length > 0 &&
            previousAssignees.split(',').includes(`${item.username}`)
          ) {
            usernameHref = previousAssignees.split(',').filter((l) => l !== item.username).join(',');
          } else {
            usernameHref = `${previousAssignees === null || previousAssignees.length === 0 ? item.username : `${previousAssignees},${item.username}`}`;
          }
          let html = `<a href=${href.replace('{username}', encodeURIComponent(usernameHref))}>
          ${previousAssignees && previousAssignees.split(',').includes(`${item.username}`) ? '<svg viewBox="0 0 16 16" class="svg octicon-check" aria-hidden="true" width="16" height="16"><path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.75.75 0 0 1 .018-1.042.75.75 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0"></path></svg>' : ''}
          <img class="ui avatar tw-align-middle" src="${htmlEscape(item.avatar_link)}" aria-hidden="true" alt="" width="20" height="20"><span class="gt-ellipsis">${htmlEscape(item.username)}</span></a>`;
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
      const labelsQuery = `&labels=${previousLabels === null || previousLabels.length === 0 ? '' : `${previousLabels}`}`;

      const noAssignee = document.getElementById('no-assignee');
      noAssignee.href = `${window.location.href.split('?')[0]}?assignees=${labelsQuery}`;
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

function initUpdateLabelHref() {
  const url = window.location.href.split('?')[0];
  const urlParams = new URLSearchParams(window.location.search);
  const previousLabels = urlParams.get('labels');
  const previousAssignees = urlParams.get('assignees');
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
      label.href = `${url}?assignees=${previousAssignees ? `${previousAssignees}` : ''}&labels=${previousLabels.split(',').filter((l) => l !== `${labelId}`).join(',')}`;
    } else {
      // otherwise add label to href
      const labelsQuery = `&labels=${previousLabels === null || previousLabels.length === 0 ? '' : `${previousLabels},`}`;
      label.href = `${url}?assignees=${previousAssignees ? `${previousAssignees}` : ''}${labelsQuery}${labelId}`;
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
  const labelsFilter = urlParams.get('labels');
  const assigneesFilter = urlParams.get('assignees');

  const cards = document.querySelectorAll('.issue-card[data-issue]');
  for (const card of cards) {
    if (!labelsFilter && !assigneesFilter) {
      // no labels and no assignee(initial state), show all cards
      card.style.display = 'flex';
      continue;
    }

    const issueLabels = [];
    if (labelsFilter) {
      for (const label of card.querySelectorAll('[data-label-id]')) {
        issueLabels.push($(label).data('label-id').toString());
      }
    }

    const labelsArray = labelsFilter ? labelsFilter.split(',') : [];
    if (!assigneesFilter && labelsArray.every((l) => issueLabels.includes(l))) {
      card.style.display = 'flex';
      continue;
    }

    const issueAssignees = [];
    if (assigneesFilter) {
      for (const assignee of card.querySelectorAll('[data-username]')) {
        issueAssignees.push($(assignee).data('username'));
      }
    }

    const assigneesArray = assigneesFilter ? assigneesFilter.split(',') : [];
    if (assigneesArray.every((a) => issueAssignees.includes(a)) && labelsArray.every((l) => issueLabels.includes(l))) {
      card.style.display = 'flex';
    }
  }
}

export function initProjectFilter() {
  if (!document.querySelectorAll('.page-content.repository.projects.view-project').length) return;
  initProjectCardFilter();
  initUpdateLabelHref();
  initUserDropdown();
}
