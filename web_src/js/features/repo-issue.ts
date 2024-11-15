import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {createTippy, showTemporaryTooltip} from '../modules/tippy.ts';
import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {setFileFolding} from './file-fold.ts';
import {ComboMarkdownEditor, getComboMarkdownEditor, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {parseIssuePageInfo, toAbsoluteUrl} from '../utils.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {initRepoIssueSidebar} from './repo-issue-sidebar.ts';

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
      if (e.key === 'Enter') {
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

/**
 * @param {HTMLElement} item
 */
function excludeLabel(item) {
  const href = item.getAttribute('href');
  const id = item.getAttribute('data-label-id');

  const regStr = `labels=((?:-?[0-9]+%2c)*)(${id})((?:%2c-?[0-9]+)*)&`;
  const newStr = 'labels=$1-$2$3&';

  window.location.assign(href.replace(new RegExp(regStr), newStr));
}

export function initRepoIssueSidebarList() {
  const issuePageInfo = parseIssuePageInfo();
  const crossRepoSearch = $('#crossRepoSearch').val();
  let issueSearchUrl = `${issuePageInfo.repoLink}/issues/search?q={query}&type=${issuePageInfo.issueDependencySearchType}`;
  if (crossRepoSearch === 'true') {
    issueSearchUrl = `${appSubUrl}/issues/search?q={query}&priority_repo_id=${issuePageInfo.repoId}&type=${issuePageInfo.issueDependencySearchType}`;
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
              name: `<div class="gt-ellipsis">#${issue.number} ${htmlEscape(issue.title)}</div>
<div class="text small tw-break-anywhere">${htmlEscape(issue.repository.full_name)}</div>`,
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

  // FIXME: it is wrong place to init ".ui.dropdown.label-filter"
  $('.menu .ui.dropdown.label-filter').on('keydown', (e) => {
    if (e.altKey && e.key === 'Enter') {
      const selectedItem = document.querySelector('.menu .ui.dropdown.label-filter .menu .item.selected');
      if (selectedItem) {
        excludeLabel(selectedItem);
      }
    }
  });
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

        document.querySelector(`#${deleteButton.getAttribute('data-comment-id')}`)?.remove();

        if (conversationHolder && !conversationHolder.querySelector('.comment')) {
          const path = conversationHolder.getAttribute('data-path');
          const side = conversationHolder.getAttribute('data-side');
          const idx = conversationHolder.getAttribute('data-idx');
          const lineType = conversationHolder.closest('tr')?.getAttribute('data-line-type');

          // the conversation holder could appear either on the "Conversation" page, or the "Files Changed" page
          // on the Conversation page, there is no parent "tr", so no need to do anything for "add-code-comment"
          if (lineType) {
            if (lineType === 'same') {
              document.querySelector(`[data-path="${path}"] .add-code-comment[data-idx="${idx}"]`).classList.remove('tw-invisible');
            } else {
              document.querySelector(`[data-path="${path}"] .add-code-comment[data-side="${side}"][data-idx="${idx}"]`).classList.remove('tw-invisible');
            }
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
      const choiceEl = $choice[0];
      const url = choiceEl.getAttribute('data-do');
      if (url) {
        const buttonText = pullUpdateButton.querySelector('.button-text');
        if (buttonText) {
          buttonText.textContent = choiceEl.textContent;
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
  const wrapper = document.querySelector('#allow-edits-from-maintainers');
  if (!wrapper) return;
  const checkbox = wrapper.querySelector('input[type="checkbox"]');
  checkbox.addEventListener('input', async () => {
    const url = `${wrapper.getAttribute('data-url')}/set_allow_maintainer_edit`;
    wrapper.classList.add('is-loading');
    try {
      const resp = await POST(url, {data: new URLSearchParams({allow_maintainer_edit: checkbox.checked})});
      if (!resp.ok) {
        throw new Error('Failed to update maintainer edit permission');
      }
      const data = await resp.json();
      checkbox.checked = data.allow_maintainer_edit;
    } catch (error) {
      checkbox.checked = !checkbox.checked;
      console.error(error);
      showTemporaryTooltip(wrapper, wrapper.getAttribute('data-prompt-error'));
    } finally {
      wrapper.classList.remove('is-loading');
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

export function initRepoIssueComments() {
  if (!$('.repository.view.issue .timeline').length) return;

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

export async function handleReply(el) {
  const form = el.closest('.comment-code-cloud').querySelector('.comment-form');
  const textarea = form.querySelector('textarea');

  hideElem(el);
  showElem(form);
  const editor = getComboMarkdownEditor(textarea) ?? await initComboMarkdownEditor(form.querySelector('.combo-markdown-editor'));
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
    await handleReply(this);
  });

  const elReviewBox = document.querySelector('.review-box-panel');
  if (elReviewBox) {
    initComboMarkdownEditor(elReviewBox.querySelector('.combo-markdown-editor'));
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
        const editor = await initComboMarkdownEditor($td[0].querySelector('.combo-markdown-editor'));
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
    const target = this.getAttribute('data-target');
    const content = document.querySelector(`#${target}`)?.textContent ?? '';
    const poster = this.getAttribute('data-poster-username');
    const reference = toAbsoluteUrl(this.getAttribute('data-reference'));
    const modalSelector = this.getAttribute('data-modal');
    const modal = document.querySelector(modalSelector);
    const textarea = modal.querySelector('textarea[name="content"]');
    textarea.value = `${content}\n\n_Originally posted by @${poster} in ${reference}_`;
    $(modal).modal('show');
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

export function initRepoIssueTitleEdit() {
  const issueTitleDisplay = document.querySelector('#issue-title-display');
  const issueTitleEditor = document.querySelector('#issue-title-editor');
  if (!issueTitleEditor) return;

  const issueTitleInput = issueTitleEditor.querySelector('input');
  const oldTitle = issueTitleInput.getAttribute('data-old-title');
  issueTitleDisplay.querySelector('#issue-title-edit-show').addEventListener('click', () => {
    hideElem(issueTitleDisplay);
    hideElem('#pull-desc-display');
    showElem(issueTitleEditor);
    showElem('#pull-desc-editor');
    if (!issueTitleInput.value.trim()) {
      issueTitleInput.value = oldTitle;
    }
    issueTitleInput.focus();
  });
  issueTitleEditor.querySelector('.ui.cancel.button').addEventListener('click', () => {
    hideElem(issueTitleEditor);
    hideElem('#pull-desc-editor');
    showElem(issueTitleDisplay);
    showElem('#pull-desc-display');
  });

  const pullDescEditor = document.querySelector('#pull-desc-editor'); // it may not exist for a merged PR
  const prTargetUpdateUrl = pullDescEditor?.getAttribute('data-target-update-url');

  const editSaveButton = issueTitleEditor.querySelector('.ui.primary.button');
  editSaveButton.addEventListener('click', async () => {
    const newTitle = issueTitleInput.value.trim();
    try {
      if (newTitle && newTitle !== oldTitle) {
        const resp = await POST(editSaveButton.getAttribute('data-update-url'), {data: new URLSearchParams({title: newTitle})});
        if (!resp.ok) {
          throw new Error(`Failed to update issue title: ${resp.statusText}`);
        }
      }
      if (prTargetUpdateUrl) {
        const newTargetBranch = document.querySelector('#pull-target-branch').getAttribute('data-branch');
        const oldTargetBranch = document.querySelector('#branch_target').textContent;
        if (newTargetBranch !== oldTargetBranch) {
          const resp = await POST(prTargetUpdateUrl, {data: new URLSearchParams({target_branch: newTargetBranch})});
          if (!resp.ok) {
            throw new Error(`Failed to update PR target branch: ${resp.statusText}`);
          }
        }
      }
      window.location.reload();
    } catch (error) {
      console.error(error);
      showErrorToast(error.message);
    }
  });
}

export function initRepoIssueBranchSelect() {
  document.querySelector('#branch-select')?.addEventListener('click', (e) => {
    const el = e.target.closest('.item[data-branch]');
    if (!el) return;
    const pullTargetBranch = document.querySelector('#pull-target-branch');
    const baseName = pullTargetBranch.getAttribute('data-basename');
    const branchNameNew = el.getAttribute('data-branch');
    const branchNameOld = pullTargetBranch.getAttribute('data-branch');
    pullTargetBranch.textContent = pullTargetBranch.textContent.replace(`${baseName}:${branchNameOld}`, `${baseName}:${branchNameNew}`);
    pullTargetBranch.setAttribute('data-branch', branchNameNew);
  });
}

async function initSingleCommentEditor($commentForm) {
  // pages:
  // * normal new issue/pr page: no status-button, no comment-button (there is only a normal submit button which can submit empty content)
  // * issue/pr view page: with comment form, has status-button and comment-button
  const editor = await initComboMarkdownEditor($commentForm[0].querySelector('.combo-markdown-editor'));
  const statusButton = document.querySelector<HTMLButtonElement>('#status-button');
  const commentButton = document.querySelector<HTMLButtonElement>('#comment-button');
  const syncUiState = () => {
    const editorText = editor.value().trim(), isUploading = editor.isUploading();
    if (statusButton) {
      statusButton.textContent = statusButton.getAttribute(editorText ? 'data-status-and-comment' : 'data-status');
      statusButton.disabled = isUploading;
    }
    if (commentButton) {
      commentButton.disabled = !editorText || isUploading;
    }
  };
  editor.container.addEventListener(ComboMarkdownEditor.EventUploadStateChanged, syncUiState);
  editor.container.addEventListener(ComboMarkdownEditor.EventEditorContentChanged, syncUiState);
  syncUiState();
}

function initIssueTemplateCommentEditors($commentForm) {
  // pages:
  // * new issue with issue template
  const $comboFields = $commentForm.find('.combo-editor-dropzone');

  const initCombo = async (elCombo) => {
    const $formField = $(elCombo.querySelector('.form-field-real'));
    const dropzoneContainer = elCombo.querySelector('.form-field-dropzone');
    const markdownEditor = elCombo.querySelector('.combo-markdown-editor');

    const editor = await initComboMarkdownEditor(markdownEditor);
    editor.container.addEventListener(ComboMarkdownEditor.EventEditorContentChanged, () => $formField.val(editor.value()));

    $formField.on('focus', async () => {
      // deactivate all markdown editors
      showElem($commentForm.find('.combo-editor-dropzone .form-field-real'));
      hideElem($commentForm.find('.combo-editor-dropzone .combo-markdown-editor'));
      hideElem($commentForm.find('.combo-editor-dropzone .form-field-dropzone'));

      // activate this markdown editor
      hideElem($formField);
      showElem(markdownEditor);
      showElem(dropzoneContainer);

      await editor.switchToUserPreference();
      editor.focus();
    });
  };

  for (const el of $comboFields) {
    initCombo(el);
  }
}

export function initRepoCommentFormAndSidebar() {
  const $commentForm = $('.comment.form');
  if (!$commentForm.length) return;

  if ($commentForm.find('.field.combo-editor-dropzone').length) {
    // at the moment, if a form has multiple combo-markdown-editors, it must be an issue template form
    initIssueTemplateCommentEditors($commentForm);
  } else if ($commentForm.find('.combo-markdown-editor').length) {
    // it's quite unclear about the "comment form" elements, sometimes it's for issue comment, sometimes it's for file editor/uploader message
    initSingleCommentEditor($commentForm);
  }

  initRepoIssueSidebar();
}
