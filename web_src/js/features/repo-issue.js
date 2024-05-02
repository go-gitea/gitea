import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {showTemporaryTooltip, createTippy} from '../modules/tippy.js';
import {hideElem, showElem, toggleElem} from '../utils/dom.js';
import {setFileFolding} from './file-fold.js';
import {getComboMarkdownEditor, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';
import {toAbsoluteUrl} from '../utils.js';
import {initDropzone} from './common-global.js';
import {POST, GET} from '../modules/fetch.js';

const {appSubUrl} = window.config;

export function initRepoIssueTimeTracking() {
  $(document).on('click', '.issue-add-time', () => {
    $('.issue-start-time-modal').modal({
      duration: 200,
      onApprove() {
        $('#add_time_manual_form').trigger('submit');
      },
    }).modal('show');
    $('.issue-start-time-modal input').on('keydown', (e) => {
      if ((e.keyCode || e.key) === 13) {
        $('#add_time_manual_form').trigger('submit');
      }
    });
  });
  $(document).on('click', '.issue-start-time, .issue-stop-time', () => {
    $('#toggle_stopwatch_form').trigger('submit');
  });
  $(document).on('click', '.issue-cancel-time', () => {
    $('#cancel_stopwatch_form').trigger('submit');
  });
  $(document).on('click', 'button.issue-delete-time', function () {
    const sel = `.issue-delete-time-modal[data-id="${$(this).data('id')}"]`;
    $(sel).modal({
      duration: 200,
      onApprove() {
        $(`${sel} form`).trigger('submit');
      },
    }).modal('show');
  });
}

async function updateDeadline(deadlineString) {
  hideElem('#deadline-err-invalid-date');
  document.getElementById('deadline-loader')?.classList.add('is-loading');

  let realDeadline = null;
  if (deadlineString !== '') {
    const newDate = Date.parse(deadlineString);

    if (Number.isNaN(newDate)) {
      document.getElementById('deadline-loader')?.classList.remove('is-loading');
      showElem('#deadline-err-invalid-date');
      return false;
    }
    realDeadline = new Date(newDate);
  }

  try {
    const response = await POST(document.getElementById('update-issue-deadline-form').getAttribute('action'), {
      data: {due_date: realDeadline},
    });

    if (response.ok) {
      window.location.reload();
    } else {
      throw new Error('Invalid response');
    }
  } catch (error) {
    console.error(error);
    document.getElementById('deadline-loader').classList.remove('is-loading');
    showElem('#deadline-err-invalid-date');
  }
}

export function initRepoIssueDue() {
  $(document).on('click', '.issue-due-edit', () => {
    toggleElem('#deadlineForm');
  });
  $(document).on('click', '.issue-due-remove', () => {
    updateDeadline('');
  });
  $(document).on('submit', '.issue-due-form', () => {
    updateDeadline($('#deadlineDate').val());
    return false;
  });
}

/**
 * @param {HTMLElement} item
 */
function excludeLabel(item) {
  const href = item.getAttribute('href');
  const id = item.getAttribute('data-label-id');

  const regStr = `labels=((?:-?[0-9]+%2c)*)(${id})((?:%2c-?[0-9]+)*)&`;
  const newStr = 'labels=$1-$2$3&';

  window.location = href.replace(new RegExp(regStr), newStr);
}

export function initRepoIssueSidebarList() {
  const repolink = $('#repolink').val();
  const repoId = $('#repoId').val();
  const crossRepoSearch = $('#crossRepoSearch').val();
  const tp = $('#type').val();
  let issueSearchUrl = `${appSubUrl}/${repolink}/issues/search?q={query}&type=${tp}`;
  if (crossRepoSearch === 'true') {
    issueSearchUrl = `${appSubUrl}/issues/search?q={query}&priority_repo_id=${repoId}&type=${tp}`;
  }
  $('#new-dependency-drop-list')
    .dropdown({
      apiSettings: {
        url: issueSearchUrl,
        onResponse(response) {
          const filteredResponse = {success: true, results: []};
          const currIssueId = $('#new-dependency-drop-list').data('issue-id');
          // Parse the response from the api to work with our dropdown
          $.each(response, (_i, issue) => {
            // Don't list current issue in the dependency list.
            if (issue.id === currIssueId) {
              return;
            }
            filteredResponse.results.push({
              name: `#${issue.number} ${htmlEscape(issue.title)
              }<div class="text small gt-word-break">${htmlEscape(issue.repository.full_name)}</div>`,
              value: issue.id,
            });
          });
          return filteredResponse;
        },
        cache: false,
      },

      fullTextSearch: true,
    });

  $('.menu a.label-filter-item').each(function () {
    $(this).on('click', function (e) {
      if (e.altKey) {
        e.preventDefault();
        excludeLabel(this);
      }
    });
  });

  $('.menu .ui.dropdown.label-filter').on('keydown', (e) => {
    if (e.altKey && e.keyCode === 13) {
      const selectedItem = document.querySelector('.menu .ui.dropdown.label-filter .menu .item.selected');
      if (selectedItem) {
        excludeLabel(selectedItem);
      }
    }
  });
  $('.ui.dropdown.label-filter, .ui.dropdown.select-label').dropdown('setting', {'hideDividers': 'empty'}).dropdown('refreshItems');
}

export function initRepoIssueCommentDelete() {
  // Delete comment
  document.addEventListener('click', async (e) => {
    if (!e.target.matches('.delete-comment')) return;
    e.preventDefault();

    const deleteButton = e.target;
    if (window.confirm(deleteButton.getAttribute('data-locale'))) {
      try {
        const response = await POST(deleteButton.getAttribute('data-url'));
        if (!response.ok) throw new Error('Failed to delete comment');

        const conversationHolder = deleteButton.closest('.conversation-holder');
        const parentTimelineItem = deleteButton.closest('.timeline-item');
        const parentTimelineGroup = deleteButton.closest('.timeline-item-group');

        // Check if this was a pending comment.
        if (conversationHolder?.querySelector('.pending-label')) {
          const counter = document.querySelector('#review-box .review-comments-counter');
          let num = parseInt(counter?.getAttribute('data-pending-comment-number')) - 1 || 0;
          num = Math.max(num, 0);
          counter.setAttribute('data-pending-comment-number', num);
          counter.textContent = String(num);
        }

        document.getElementById(deleteButton.getAttribute('data-comment-id'))?.remove();

        if (conversationHolder && !conversationHolder.querySelector('.comment')) {
          const path = conversationHolder.getAttribute('data-path');
          const side = conversationHolder.getAttribute('data-side');
          const idx = conversationHolder.getAttribute('data-idx');
          const lineType = conversationHolder.closest('tr').getAttribute('data-line-type');

          if (lineType === 'same') {
            document.querySelector(`[data-path="${path}"] .add-code-comment[data-idx="${idx}"]`).classList.remove('tw-invisible');
          } else {
            document.querySelector(`[data-path="${path}"] .add-code-comment[data-side="${side}"][data-idx="${idx}"]`).classList.remove('tw-invisible');
          }

          conversationHolder.remove();
        }

        // Check if there is no review content, move the time avatar upward to avoid overlapping the content below.
        if (!parentTimelineGroup?.querySelector('.timeline-item.comment') && !parentTimelineItem?.querySelector('.conversation-holder')) {
          const timelineAvatar = parentTimelineGroup?.querySelector('.timeline-avatar');
          timelineAvatar?.classList.remove('timeline-avatar-offset');
        }
      } catch (error) {
        console.error(error);
      }
    }
  });
}

export function initRepoIssueDependencyDelete() {
  // Delete Issue dependency
  $(document).on('click', '.delete-dependency-button', (e) => {
    const id = e.currentTarget.getAttribute('data-id');
    const type = e.currentTarget.getAttribute('data-type');

    $('.remove-dependency').modal({
      closable: false,
      duration: 200,
      onApprove: () => {
        $('#removeDependencyID').val(id);
        $('#dependencyType').val(type);
        $('#removeDependencyForm').trigger('submit');
      },
    }).modal('show');
  });
}

export function initRepoIssueCodeCommentCancel() {
  // Cancel inline code comment
  document.addEventListener('click', (e) => {
    if (!e.target.matches('.cancel-code-comment')) return;

    const form = e.target.closest('form');
    if (form?.classList.contains('comment-form')) {
      hideElem(form);
      showElem(form.closest('.comment-code-cloud')?.querySelectorAll('button.comment-form-reply'));
    } else {
      form.closest('.comment-code-cloud')?.remove();
    }
  });
}

export function initRepoPullRequestUpdate() {
  // Pull Request update button
  const pullUpdateButton = document.querySelector('.update-button > button');
  if (!pullUpdateButton) return;

  pullUpdateButton.addEventListener('click', async function (e) {
    e.preventDefault();
    const redirect = this.getAttribute('data-redirect');
    this.classList.add('is-loading');
    let response;
    try {
      response = await POST(this.getAttribute('data-do'));
    } catch (error) {
      console.error(error);
    } finally {
      this.classList.remove('is-loading');
    }
    let data;
    try {
      data = await response?.json(); // the response is probably not a JSON
    } catch (error) {
      console.error(error);
    }
    if (data?.redirect) {
      window.location.href = data.redirect;
    } else if (redirect) {
      window.location.href = redirect;
    } else {
      window.location.reload();
    }
  });

  $('.update-button > .dropdown').dropdown({
    onChange(_text, _value, $choice) {
      const url = $choice[0].getAttribute('data-do');
      if (url) {
        const buttonText = pullUpdateButton.querySelector('.button-text');
        if (buttonText) {
          buttonText.textContent = $choice.text();
        }
        pullUpdateButton.setAttribute('data-do', url);
      }
    },
  });
}

export function initRepoPullRequestMergeInstruction() {
  $('.show-instruction').on('click', () => {
    toggleElem($('.instruct-content'));
  });
}

export function initRepoPullRequestAllowMaintainerEdit() {
  const wrapper = document.getElementById('allow-edits-from-maintainers');
  if (!wrapper) return;

  wrapper.querySelector('input[type="checkbox"]')?.addEventListener('change', async (e) => {
    const checked = e.target.checked;
    const url = `${wrapper.getAttribute('data-url')}/set_allow_maintainer_edit`;
    wrapper.classList.add('is-loading');
    e.target.disabled = true;
    try {
      const response = await POST(url, {data: {allow_maintainer_edit: checked}});
      if (!response.ok) {
        throw new Error('Failed to update maintainer edit permission');
      }
    } catch (error) {
      console.error(error);
      showTemporaryTooltip(wrapper, wrapper.getAttribute('data-prompt-error'));
    } finally {
      wrapper.classList.remove('is-loading');
      e.target.disabled = false;
    }
  });
}

export function initRepoIssueReferenceRepositorySearch() {
  $('.issue_reference_repository_search')
    .dropdown({
      apiSettings: {
        url: `${appSubUrl}/repo/search?q={query}&limit=20`,
        onResponse(response) {
          const filteredResponse = {success: true, results: []};
          $.each(response.data, (_r, repo) => {
            filteredResponse.results.push({
              name: htmlEscape(repo.repository.full_name),
              value: repo.repository.full_name,
            });
          });
          return filteredResponse;
        },
        cache: false,
      },
      onChange(_value, _text, $choice) {
        const $form = $choice.closest('form');
        if (!$form.length) return;

        $form[0].setAttribute('action', `${appSubUrl}/${_text}/issues/new`);
      },
      fullTextSearch: true,
    });
}

export function initRepoIssueWipTitle() {
  $('.title_wip_desc > a').on('click', (e) => {
    e.preventDefault();

    const $issueTitle = $('#issue_title');
    $issueTitle.trigger('focus');
    const value = $issueTitle.val().trim().toUpperCase();

    const wipPrefixes = $('.title_wip_desc').data('wip-prefixes');
    for (const prefix of wipPrefixes) {
      if (value.startsWith(prefix.toUpperCase())) {
        return;
      }
    }

    $issueTitle.val(`${wipPrefixes[0]} ${$issueTitle.val()}`);
  });
}

export async function updateIssuesMeta(url, action, issue_ids, id) {
  try {
    const response = await POST(url, {data: new URLSearchParams({action, issue_ids, id})});
    if (!response.ok) {
      throw new Error('Failed to update issues meta');
    }
  } catch (error) {
    console.error(error);
  }
}

export function initRepoIssueComments() {
  if (!$('.repository.view.issue .timeline').length) return;

  $('.re-request-review').on('click', async function (e) {
    e.preventDefault();
    const url = this.getAttribute('data-update-url');
    const issueId = this.getAttribute('data-issue-id');
    const id = this.getAttribute('data-id');
    const isChecked = this.classList.contains('checked');

    await updateIssuesMeta(url, isChecked ? 'detach' : 'attach', issueId, id);
    window.location.reload();
  });

  document.addEventListener('click', (e) => {
    const urlTarget = document.querySelector(':target');
    if (!urlTarget) return;

    const urlTargetId = urlTarget.id;
    if (!urlTargetId) return;

    if (!/^(issue|pull)(comment)?-\d+$/.test(urlTargetId)) return;

    if (!e.target.closest(`#${urlTargetId}`)) {
      const scrollPosition = $(window).scrollTop();
      window.location.hash = '';
      $(window).scrollTop(scrollPosition);
      window.history.pushState(null, null, ' ');
    }
  });
}

export async function handleReply($el) {
  hideElem($el);
  const $form = $el.closest('.comment-code-cloud').find('.comment-form');
  showElem($form);

  const $textarea = $form.find('textarea');
  let editor = getComboMarkdownEditor($textarea);
  if (!editor) {
    // FIXME: the initialization of the dropzone is not consistent.
    // When the page is loaded, the dropzone is initialized by initGlobalDropzone, but the editor is not initialized.
    // When the form is submitted and partially reload, none of them is initialized.
    const dropzone = $form.find('.dropzone')[0];
    if (!dropzone.dropzone) initDropzone(dropzone);
    editor = await initComboMarkdownEditor($form.find('.combo-markdown-editor'));
  }
  editor.focus();
  return editor;
}

export function initRepoPullRequestReview() {
  if (window.location.hash && window.location.hash.startsWith('#issuecomment-')) {
    // set scrollRestoration to 'manual' when there is a hash in url, so that the scroll position will not be remembered after refreshing
    if (window.history.scrollRestoration !== 'manual') {
      window.history.scrollRestoration = 'manual';
    }
    const commentDiv = document.querySelector(window.location.hash);
    if (commentDiv) {
      // get the name of the parent id
      const groupID = commentDiv.closest('div[id^="code-comments-"]')?.getAttribute('id');
      if (groupID && groupID.startsWith('code-comments-')) {
        const id = groupID.slice(14);
        const ancestorDiffBox = commentDiv.closest('.diff-file-box');
        // on pages like conversation, there is no diff header
        const diffHeader = ancestorDiffBox?.querySelector('.diff-file-header');

        // offset is for scrolling
        let offset = 30;
        if (diffHeader) {
          offset += $('.diff-detail-box').outerHeight() + $(diffHeader).outerHeight();
        }

        hideElem(`#show-outdated-${id}`);
        showElem(`#code-comments-${id}, #code-preview-${id}, #hide-outdated-${id}`);
        // if the comment box is folded, expand it
        if (ancestorDiffBox?.getAttribute('data-folded') === 'true') {
          setFileFolding(ancestorDiffBox, ancestorDiffBox.querySelector('.fold-file'), false);
        }

        window.scrollTo({
          top: $(commentDiv).offset().top - offset,
          behavior: 'instant',
        });
      }
    }
  }

  $(document).on('click', '.show-outdated', function (e) {
    e.preventDefault();
    const id = this.getAttribute('data-comment');
    hideElem(this);
    showElem(`#code-comments-${id}`);
    showElem(`#code-preview-${id}`);
    showElem(`#hide-outdated-${id}`);
  });

  $(document).on('click', '.hide-outdated', function (e) {
    e.preventDefault();
    const id = this.getAttribute('data-comment');
    hideElem(this);
    hideElem(`#code-comments-${id}`);
    hideElem(`#code-preview-${id}`);
    showElem(`#show-outdated-${id}`);
  });

  $(document).on('click', 'button.comment-form-reply', async function (e) {
    e.preventDefault();
    await handleReply($(this));
  });

  const $reviewBox = $('.review-box-panel');
  if ($reviewBox.length === 1) {
    const _promise = initComboMarkdownEditor($reviewBox.find('.combo-markdown-editor'));
  }

  // The following part is only for diff views
  if (!$('.repository.pull.diff').length) return;

  const $reviewBtn = $('.js-btn-review');
  const $panel = $reviewBtn.parent().find('.review-box-panel');
  const $closeBtn = $panel.find('.close');

  if ($reviewBtn.length && $panel.length) {
    const tippy = createTippy($reviewBtn[0], {
      content: $panel[0],
      theme: 'default',
      placement: 'bottom',
      trigger: 'click',
      maxWidth: 'none',
      interactive: true,
      hideOnClick: true,
    });

    $closeBtn.on('click', (e) => {
      e.preventDefault();
      tippy.hide();
    });
  }

  $(document).on('click', '.add-code-comment', async function (e) {
    if (e.target.classList.contains('btn-add-single')) return; // https://github.com/go-gitea/gitea/issues/4745
    e.preventDefault();

    const isSplit = this.closest('.code-diff')?.classList.contains('code-diff-split');
    const side = this.getAttribute('data-side');
    const idx = this.getAttribute('data-idx');
    const path = this.closest('[data-path]')?.getAttribute('data-path');
    const tr = this.closest('tr');
    const lineType = tr.getAttribute('data-line-type');

    const ntr = tr.nextElementSibling;
    let $ntr = $(ntr);
    if (!ntr?.classList.contains('add-comment')) {
      $ntr = $(`
        <tr class="add-comment" data-line-type="${lineType}">
          ${isSplit ? `
            <td class="add-comment-left" colspan="4"></td>
            <td class="add-comment-right" colspan="4"></td>
          ` : `
            <td class="add-comment-left add-comment-right" colspan="5"></td>
          `}
        </tr>`);
      $(tr).after($ntr);
    }

    const $td = $ntr.find(`.add-comment-${side}`);
    const $commentCloud = $td.find('.comment-code-cloud');
    if (!$commentCloud.length && !$ntr.find('button[name="pending_review"]').length) {
      try {
        const response = await GET(this.closest('[data-new-comment-url]')?.getAttribute('data-new-comment-url'));
        const html = await response.text();
        $td.html(html);
        $td.find("input[name='line']").val(idx);
        $td.find("input[name='side']").val(side === 'left' ? 'previous' : 'proposed');
        $td.find("input[name='path']").val(path);

        initDropzone($td.find('.dropzone')[0]);
        const editor = await initComboMarkdownEditor($td.find('.combo-markdown-editor'));
        editor.focus();
      } catch (error) {
        console.error(error);
      }
    }
  });
}

export function initRepoIssueReferenceIssue() {
  // Reference issue
  $(document).on('click', '.reference-issue', function (event) {
    const $this = $(this);
    const content = $(`#${$this.data('target')}`).text();
    const poster = $this.data('poster-username');
    const reference = toAbsoluteUrl($this.data('reference'));
    const $modal = $($this.data('modal'));
    $modal.find('textarea[name="content"]').val(`${content}\n\n_Originally posted by @${poster} in ${reference}_`);
    $modal.modal('show');

    event.preventDefault();
  });
}

export function initRepoIssueWipToggle() {
  // Toggle WIP
  $('.toggle-wip a, .toggle-wip button').on('click', async (e) => {
    e.preventDefault();
    const toggleWip = e.currentTarget.closest('.toggle-wip');
    const title = toggleWip.getAttribute('data-title');
    const wipPrefix = toggleWip.getAttribute('data-wip-prefix');
    const updateUrl = toggleWip.getAttribute('data-update-url');

    try {
      const params = new URLSearchParams();
      params.append('title', title?.startsWith(wipPrefix) ? title.slice(wipPrefix.length).trim() : `${wipPrefix.trim()} ${title}`);

      const response = await POST(updateUrl, {data: params});
      if (!response.ok) {
        throw new Error('Failed to toggle WIP status');
      }
      window.location.reload();
    } catch (error) {
      console.error(error);
    }
  });
}

async function pullrequest_targetbranch_change(update_url) {
  const targetBranch = $('#pull-target-branch').data('branch');
  const $branchTarget = $('#branch_target');
  if (targetBranch === $branchTarget.text()) {
    window.location.reload();
    return false;
  }
  try {
    await POST(update_url, {data: new URLSearchParams({target_branch: targetBranch})});
  } catch (error) {
    console.error(error);
  } finally {
    window.location.reload();
  }
}

export function initRepoIssueTitleEdit() {
  // Edit issue title
  const $issueTitle = $('#issue-title');
  const $editInput = $('#edit-title-input input');

  const editTitleToggle = function () {
    toggleElem($issueTitle);
    toggleElem('.not-in-edit');
    toggleElem('#edit-title-input');
    toggleElem('#pull-desc');
    toggleElem('#pull-desc-edit');
    toggleElem('.in-edit');
    toggleElem('.new-issue-button');
    document.getElementById('issue-title-wrapper')?.classList.toggle('edit-active');
    $editInput[0].focus();
    $editInput[0].select();
    return false;
  };

  $('#edit-title').on('click', editTitleToggle);
  $('#cancel-edit-title').on('click', editTitleToggle);
  $('#save-edit-title').on('click', editTitleToggle).on('click', async function () {
    const pullrequest_target_update_url = this.getAttribute('data-target-update-url');
    if (!$editInput.val().length || $editInput.val() === $issueTitle.text()) {
      $editInput.val($issueTitle.text());
      await pullrequest_targetbranch_change(pullrequest_target_update_url);
    } else {
      try {
        const params = new URLSearchParams();
        params.append('title', $editInput.val());
        const response = await POST(this.getAttribute('data-update-url'), {data: params});
        const data = await response.json();
        $editInput.val(data.title);
        $issueTitle.text(data.title);
        if (pullrequest_target_update_url) {
          await pullrequest_targetbranch_change(pullrequest_target_update_url); // it will reload the window
        } else {
          window.location.reload();
        }
      } catch (error) {
        console.error(error);
      }
    }
    return false;
  });
}

export function initRepoIssueBranchSelect() {
  const changeBranchSelect = function () {
    const $selectionTextField = $('#pull-target-branch');

    const baseName = $selectionTextField.data('basename');
    const branchNameNew = $(this).data('branch');
    const branchNameOld = $selectionTextField.data('branch');

    // Replace branch name to keep translation from HTML template
    $selectionTextField.html($selectionTextField.html().replace(
      `${baseName}:${branchNameOld}`,
      `${baseName}:${branchNameNew}`,
    ));
    $selectionTextField.data('branch', branchNameNew); // update branch name in setting
  };
  $('#branch-select > .item').on('click', changeBranchSelect);
}

export function initSingleCommentEditor($commentForm) {
  // pages:
  // * normal new issue/pr page, no status-button
  // * issue/pr view page, with comment form, has status-button
  const opts = {};
  const statusButton = document.getElementById('status-button');
  if (statusButton) {
    opts.onContentChanged = (editor) => {
      const statusText = statusButton.getAttribute(editor.value().trim() ? 'data-status-and-comment' : 'data-status');
      statusButton.textContent = statusText;
    };
  }
  initComboMarkdownEditor($commentForm.find('.combo-markdown-editor'), opts);
}

export function initIssueTemplateCommentEditors($commentForm) {
  // pages:
  // * new issue with issue template
  const $comboFields = $commentForm.find('.combo-editor-dropzone');

  const initCombo = async ($combo) => {
    const $dropzoneContainer = $combo.find('.form-field-dropzone');
    const $formField = $combo.find('.form-field-real');
    const $markdownEditor = $combo.find('.combo-markdown-editor');

    const editor = await initComboMarkdownEditor($markdownEditor, {
      onContentChanged: (editor) => {
        $formField.val(editor.value());
      },
    });

    $formField.on('focus', async () => {
      // deactivate all markdown editors
      showElem($commentForm.find('.combo-editor-dropzone .form-field-real'));
      hideElem($commentForm.find('.combo-editor-dropzone .combo-markdown-editor'));
      hideElem($commentForm.find('.combo-editor-dropzone .form-field-dropzone'));

      // activate this markdown editor
      hideElem($formField);
      showElem($markdownEditor);
      showElem($dropzoneContainer);

      await editor.switchToUserPreference();
      editor.focus();
    });
  };

  for (const el of $comboFields) {
    initCombo($(el));
  }
}

// This function used to show and hide archived label on issue/pr
//  page in the sidebar where we select the labels
//  If we have any archived label tagged to issue and pr. We will show that
//  archived label with checked classed otherwise we will hide it
//  with the help of this function.
//  This function runs globally.
export function initArchivedLabelHandler() {
  if (!document.querySelector('.archived-label-hint')) return;
  for (const label of document.querySelectorAll('[data-is-archived]')) {
    toggleElem(label, label.classList.contains('checked'));
  }
}
