import $ from 'jquery';
import {POST} from '../modules/fetch.ts';
import {updateIssuesMeta} from './repo-common.ts';
import {svg} from '../svg.ts';
import {htmlEscape} from 'escape-goat';
import {toggleElem} from '../utils/dom.ts';

// if there are draft comments, confirm before reloading, to avoid losing comments
function reloadConfirmDraftComment() {
  const commentTextareas = [
    document.querySelector('.edit-content-zone:not(.tw-hidden) textarea'),
    document.querySelector('#comment-form textarea'),
  ];
  for (const textarea of commentTextareas) {
    // Most users won't feel too sad if they lose a comment with 10 chars, they can re-type these in seconds.
    // But if they have typed more (like 50) chars and the comment is lost, they will be very unhappy.
    if (textarea && textarea.value.trim().length > 10) {
      textarea.parentElement.scrollIntoView();
      if (!window.confirm('Page will be reloaded, but there are draft comments. Continuing to reload will discard the comments. Continue?')) {
        return;
      }
      break;
    }
  }
  window.location.reload();
}

function initBranchSelector() {
  const elSelectBranch = document.querySelector('.ui.dropdown.select-branch');
  if (!elSelectBranch) return;

  const urlUpdateIssueRef = elSelectBranch.getAttribute('data-url-update-issueref');
  const $selectBranch = $(elSelectBranch);
  const $branchMenu = $selectBranch.find('.reference-list-menu');
  $branchMenu.find('.item:not(.no-select)').on('click', async function (e) {
    e.preventDefault();
    const selectedValue = this.getAttribute('data-id'); // eg: "refs/heads/my-branch"
    const selectedText = this.getAttribute('data-name'); // eg: "my-branch"
    if (urlUpdateIssueRef) {
      // for existing issue, send request to update issue ref, and reload page
      try {
        await POST(urlUpdateIssueRef, {data: new URLSearchParams({ref: selectedValue})});
        window.location.reload();
      } catch (error) {
        console.error(error);
      }
    } else {
      // for new issue, only update UI&form, do not send request/reload
      const selectedHiddenSelector = this.getAttribute('data-id-selector');
      document.querySelector(selectedHiddenSelector).value = selectedValue;
      elSelectBranch.querySelector('.text-branch-name').textContent = selectedText;
    }
  });
}

// List submits
function initListSubmits(selector, outerSelector) {
  const $list = $(`.ui.${outerSelector}.list`);
  const $noSelect = $list.find('.no-select');
  const $listMenu = $(`.${selector} .menu`);
  let hasUpdateAction = $listMenu.data('action') === 'update';
  const items = {};

  $(`.${selector}`).dropdown({
    'action': 'nothing', // do not hide the menu if user presses Enter
    fullTextSearch: 'exact',
    async onHide() {
      hasUpdateAction = $listMenu.data('action') === 'update'; // Update the var
      if (hasUpdateAction) {
        // TODO: Add batch functionality and make this 1 network request.
        const itemEntries = Object.entries(items);
        for (const [elementId, item] of itemEntries) {
          await updateIssuesMeta(
            item['update-url'],
            item.action,
            item['issue-id'],
            elementId,
          );
        }
        if (itemEntries.length) {
          reloadConfirmDraftComment();
        }
      }
    },
  });

  $listMenu.find('.item:not(.no-select)').on('click', function (e) {
    e.preventDefault();
    if (this.classList.contains('ban-change')) {
      return false;
    }

    hasUpdateAction = $listMenu.data('action') === 'update'; // Update the var

    const clickedItem = this; // eslint-disable-line unicorn/no-this-assignment
    const scope = this.getAttribute('data-scope');

    $(this).parent().find('.item').each(function () {
      if (scope) {
        // Enable only clicked item for scoped labels
        if (this.getAttribute('data-scope') !== scope) {
          return true;
        }
        if (this !== clickedItem && !this.classList.contains('checked')) {
          return true;
        }
      } else if (this !== clickedItem) {
        // Toggle for other labels
        return true;
      }

      if (this.classList.contains('checked')) {
        $(this).removeClass('checked');
        $(this).find('.octicon-check').addClass('tw-invisible');
        if (hasUpdateAction) {
          if (!($(this).data('id') in items)) {
            items[$(this).data('id')] = {
              'update-url': $listMenu.data('update-url'),
              action: 'detach',
              'issue-id': $listMenu.data('issue-id'),
            };
          } else {
            delete items[$(this).data('id')];
          }
        }
      } else {
        $(this).addClass('checked');
        $(this).find('.octicon-check').removeClass('tw-invisible');
        if (hasUpdateAction) {
          if (!($(this).data('id') in items)) {
            items[$(this).data('id')] = {
              'update-url': $listMenu.data('update-url'),
              action: 'attach',
              'issue-id': $listMenu.data('issue-id'),
            };
          } else {
            delete items[$(this).data('id')];
          }
        }
      }
    });

    // TODO: Which thing should be done for choosing review requests
    // to make chosen items be shown on time here?
    if (selector === 'select-reviewers-modify' || selector === 'select-assignees-modify') {
      return false;
    }

    const listIds = [];
    $(this).parent().find('.item').each(function () {
      if (this.classList.contains('checked')) {
        listIds.push($(this).data('id'));
        $($(this).data('id-selector')).removeClass('tw-hidden');
      } else {
        $($(this).data('id-selector')).addClass('tw-hidden');
      }
    });
    if (!listIds.length) {
      $noSelect.removeClass('tw-hidden');
    } else {
      $noSelect.addClass('tw-hidden');
    }
    $($(this).parent().data('id')).val(listIds.join(','));
    return false;
  });
  $listMenu.find('.no-select.item').on('click', function (e) {
    e.preventDefault();
    if (hasUpdateAction) {
      (async () => {
        await updateIssuesMeta(
          $listMenu.data('update-url'),
          'clear',
          $listMenu.data('issue-id'),
          '',
        );
        reloadConfirmDraftComment();
      })();
    }

    $(this).parent().find('.item').each(function () {
      $(this).removeClass('checked');
      $(this).find('.octicon-check').addClass('tw-invisible');
    });

    if (selector === 'select-reviewers-modify' || selector === 'select-assignees-modify') {
      return false;
    }

    $list.find('.item').each(function () {
      $(this).addClass('tw-hidden');
    });
    $noSelect.removeClass('tw-hidden');
    $($(this).parent().data('id')).val('');
  });
}

function selectItem(select_id, input_id) {
  const $menu = $(`${select_id} .menu`);
  const $list = $(`.ui${select_id}.list`);
  const hasUpdateAction = $menu.data('action') === 'update';

  $menu.find('.item:not(.no-select)').on('click', function () {
    $(this).parent().find('.item').each(function () {
      $(this).removeClass('selected active');
    });

    $(this).addClass('selected active');
    if (hasUpdateAction) {
      (async () => {
        await updateIssuesMeta(
          $menu.data('update-url'),
          '',
          $menu.data('issue-id'),
          $(this).data('id'),
        );
        reloadConfirmDraftComment();
      })();
    }

    let icon = '';
    if (input_id === '#milestone_id') {
      icon = svg('octicon-milestone', 18, 'tw-mr-2');
    } else if (input_id === '#project_id') {
      icon = svg('octicon-project', 18, 'tw-mr-2');
    } else if (input_id === '#assignee_id') {
      icon = `<img class="ui avatar image tw-mr-2" alt="avatar" src=${$(this).data('avatar')}>`;
    }

    $list.find('.selected').html(`
        <a class="item muted sidebar-item-link" href="${htmlEscape(this.getAttribute('data-href'))}">
          ${icon}
          ${htmlEscape(this.textContent)}
        </a>
      `);

    $(`.ui${select_id}.list .no-select`).addClass('tw-hidden');
    $(input_id).val($(this).data('id'));
  });
  $menu.find('.no-select.item').on('click', function () {
    $(this).parent().find('.item:not(.no-select)').each(function () {
      $(this).removeClass('selected active');
    });

    if (hasUpdateAction) {
      (async () => {
        await updateIssuesMeta(
          $menu.data('update-url'),
          '',
          $menu.data('issue-id'),
          $(this).data('id'),
        );
        reloadConfirmDraftComment();
      })();
    }

    $list.find('.selected').html('');
    $list.find('.no-select').removeClass('tw-hidden');
    $(input_id).val('');
  });
}

function initRepoIssueDue() {
  const form = document.querySelector<HTMLFormElement>('.issue-due-form');
  if (!form) return;
  const deadline = form.querySelector<HTMLInputElement>('input[name=deadline]');
  document.querySelector('.issue-due-edit')?.addEventListener('click', () => {
    toggleElem(form);
  });
  document.querySelector('.issue-due-remove')?.addEventListener('click', () => {
    deadline.value = '';
    form.dispatchEvent(new Event('submit', {cancelable: true, bubbles: true}));
  });
}

export function initRepoIssueSidebar() {
  initBranchSelector();
  initRepoIssueDue();

  // Init labels and assignees
  initListSubmits('select-label', 'labels');
  initListSubmits('select-assignees', 'assignees');
  initListSubmits('select-assignees-modify', 'assignees');
  initListSubmits('select-reviewers-modify', 'assignees');

  // Milestone, Assignee, Project
  selectItem('.select-project', '#project_id');
  selectItem('.select-milestone', '#milestone_id');
  selectItem('.select-assignee', '#assignee_id');
}
