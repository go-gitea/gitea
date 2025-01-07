import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {createTippy, showTemporaryTooltip} from '../modules/tippy.ts';
import {
  addDelegatedEventListener,
  createElementFromHTML,
  hideElem,
  queryElems,
  showElem,
  toggleElem,
  type DOMEvent,
} from '../utils/dom.ts';
import {setFileFolding} from './file-fold.ts';
import {ComboMarkdownEditor, getComboMarkdownEditor, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {parseIssuePageInfo, toAbsoluteUrl} from '../utils.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {initRepoIssueSidebar} from './repo-issue-sidebar.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {ignoreAreYouSure} from '../vendor/jquery.are-you-sure.ts';

const {appSubUrl} = window.config;

export function initRepoIssueSidebarList() {
  const issuePageInfo = parseIssuePageInfo();
  const crossRepoSearch = $('#crossRepoSearch').val();
  let issueSearchUrl = `${issuePageInfo.repoLink}/issues/search?q={query}&type=${issuePageInfo.issueDependencySearchType}`;
  if (crossRepoSearch === 'true') {
    issueSearchUrl = `${appSubUrl}/issues/search?q={query}&priority_repo_id=${issuePageInfo.repoId}&type=${issuePageInfo.issueDependencySearchType}`;
  }
  fomanticQuery('#new-dependency-drop-list').dropdown({
    fullTextSearch: true,
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
  });
}

function initRepoIssueLabelFilter(elDropdown: HTMLElement) {
  const url = new URL(window.location.href);
  const showArchivedLabels = url.searchParams.get('archived_labels') === 'true';
  const queryLabels = url.searchParams.get('labels') || '';
  const selectedLabelIds = new Set<string>();
  for (const id of queryLabels ? queryLabels.split(',') : []) {
    selectedLabelIds.add(`${Math.abs(parseInt(id))}`); // "labels" contains negative ids, which are excluded
  }

  const excludeLabel = (e: MouseEvent|KeyboardEvent, item: Element) => {
    e.preventDefault();
    e.stopPropagation();
    const labelId = item.getAttribute('data-label-id');
    let labelIds: string[] = queryLabels ? queryLabels.split(',') : [];
    labelIds = labelIds.filter((id) => Math.abs(parseInt(id)) !== Math.abs(parseInt(labelId)));
    labelIds.push(`-${labelId}`);
    url.searchParams.set('labels', labelIds.join(','));
    window.location.assign(url);
  };

  // alt(or option) + click to exclude label
  queryElems(elDropdown, '.label-filter-query-item', (el) => {
    el.addEventListener('click', (e: MouseEvent) => {
      if (e.altKey) excludeLabel(e, el);
    });
  });
  // alt(or option) + enter to exclude selected label
  elDropdown.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.altKey && e.key === 'Enter') {
      const selectedItem = elDropdown.querySelector('.label-filter-query-item.selected');
      if (selectedItem) excludeLabel(e, selectedItem);
    }
  });
  // no "labels" query parameter means "all issues"
  elDropdown.querySelector('.label-filter-query-default').classList.toggle('selected', queryLabels === '');
  // "labels=0" query parameter means "issues without label"
  elDropdown.querySelector('.label-filter-query-not-set').classList.toggle('selected', queryLabels === '0');

  // prepare to process "archived" labels
  const elShowArchivedLabel = elDropdown.querySelector('.label-filter-archived-toggle');
  if (!elShowArchivedLabel) return;
  const elShowArchivedInput = elShowArchivedLabel.querySelector<HTMLInputElement>('input');
  elShowArchivedInput.checked = showArchivedLabels;
  const archivedLabels = elDropdown.querySelectorAll('.item[data-is-archived]');
  // if no archived labels, hide the toggle and return
  if (!archivedLabels.length) {
    hideElem(elShowArchivedLabel);
    return;
  }

  // show the archived labels if the toggle is checked or the label is selected
  for (const label of archivedLabels) {
    toggleElem(label, showArchivedLabels || selectedLabelIds.has(label.getAttribute('data-label-id')));
  }
  // update the url when the toggle is changed and reload
  elShowArchivedInput.addEventListener('input', () => {
    if (elShowArchivedInput.checked) {
      url.searchParams.set('archived_labels', 'true');
    } else {
      url.searchParams.delete('archived_labels');
    }
    window.location.assign(url);
  });
}

export function initRepoIssueFilterItemLabel() {
  // the "label-filter" is used in 2 templates: projects/view, issue/filter_list (issue list page including the milestone page)
  queryElems(document, '.ui.dropdown.label-filter', initRepoIssueLabelFilter);
}

export function initRepoIssueCommentDelete() {
  // Delete comment
  document.addEventListener('click', async (e: DOMEvent<MouseEvent>) => {
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
          counter.setAttribute('data-pending-comment-number', String(num));
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
  document.addEventListener('click', (e: DOMEvent<MouseEvent>) => {
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
    let response: Response;
    try {
      response = await POST(this.getAttribute('data-do'));
    } catch (error) {
      console.error(error);
    } finally {
      this.classList.remove('is-loading');
    }
    let data: Record<string, any>;
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
  const checkbox = wrapper.querySelector<HTMLInputElement>('input[type="checkbox"]');
  checkbox.addEventListener('input', async () => {
    const url = `${wrapper.getAttribute('data-url')}/set_allow_maintainer_edit`;
    wrapper.classList.add('is-loading');
    try {
      const resp = await POST(url, {data: new URLSearchParams({
        allow_maintainer_edit: String(checkbox.checked),
      })});
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
    const value = ($issueTitle.val() as string).trim().toUpperCase();

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

  document.addEventListener('click', (e: DOMEvent<MouseEvent>) => {
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
    const commentDiv = document.querySelector(window.location.hash);
    if (commentDiv) {
      // get the name of the parent id
      const groupID = commentDiv.closest('div[id^="code-comments-"]')?.getAttribute('id');
      if (groupID && groupID.startsWith('code-comments-')) {
        const id = groupID.slice(14);
        const ancestorDiffBox = commentDiv.closest('.diff-file-box');

        hideElem(`#show-outdated-${id}`);
        showElem(`#code-comments-${id}, #code-preview-${id}, #hide-outdated-${id}`);
        // if the comment box is folded, expand it
        if (ancestorDiffBox?.getAttribute('data-folded') === 'true') {
          setFileFolding(ancestorDiffBox, ancestorDiffBox.querySelector('.fold-file'), false);
        }
      }
      // set scrollRestoration to 'manual' when there is a hash in url, so that the scroll position will not be remembered after refreshing
      if (window.history.scrollRestoration !== 'manual') window.history.scrollRestoration = 'manual';
      // wait for a while because some elements (eg: image, editor, etc.) may change the viewport's height.
      setTimeout(() => commentDiv.scrollIntoView({block: 'start'}), 100);
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

  addDelegatedEventListener(document, 'click', '.add-code-comment', async (el, e) => {
    e.preventDefault();

    const isSplit = el.closest('.code-diff')?.classList.contains('code-diff-split');
    const side = el.getAttribute('data-side');
    const idx = el.getAttribute('data-idx');
    const path = el.closest('[data-path]')?.getAttribute('data-path');
    const tr = el.closest('tr');
    const lineType = tr.getAttribute('data-line-type');

    let ntr = tr.nextElementSibling;
    if (!ntr?.classList.contains('add-comment')) {
      ntr = createElementFromHTML(`
        <tr class="add-comment" data-line-type="${lineType}">
          ${isSplit ? `
            <td class="add-comment-left" colspan="4"></td>
            <td class="add-comment-right" colspan="4"></td>
          ` : `
            <td class="add-comment-left add-comment-right" colspan="5"></td>
          `}
        </tr>`);
      tr.after(ntr);
    }
    const td = ntr.querySelector(`.add-comment-${side}`);
    const commentCloud = td.querySelector('.comment-code-cloud');
    if (!commentCloud && !ntr.querySelector('button[name="pending_review"]')) {
      const response = await GET(el.closest('[data-new-comment-url]')?.getAttribute('data-new-comment-url'));
      td.innerHTML = await response.text();
      td.querySelector<HTMLInputElement>("input[name='line']").value = idx;
      td.querySelector<HTMLInputElement>("input[name='side']").value = (side === 'left' ? 'previous' : 'proposed');
      td.querySelector<HTMLInputElement>("input[name='path']").value = path;
      const editor = await initComboMarkdownEditor(td.querySelector<HTMLElement>('.combo-markdown-editor'));
      editor.focus();
    }
  });
}

export function initRepoIssueReferenceIssue() {
  // Reference issue
  $(document).on('click', '.reference-issue', function (e) {
    const target = this.getAttribute('data-target');
    const content = document.querySelector(`#${target}`)?.textContent ?? '';
    const poster = this.getAttribute('data-poster-username');
    const reference = toAbsoluteUrl(this.getAttribute('data-reference'));
    const modalSelector = this.getAttribute('data-modal');
    const modal = document.querySelector(modalSelector);
    const textarea = modal.querySelector('textarea[name="content"]');
    textarea.value = `${content}\n\n_Originally posted by @${poster} in ${reference}_`;
    $(modal).modal('show');
    e.preventDefault();
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
  const issueTitleEditor = document.querySelector<HTMLFormElement>('#issue-title-editor');
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
  issueTitleEditor.addEventListener('submit', async (e) => {
    e.preventDefault();
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
      ignoreAreYouSure(issueTitleEditor);
      window.location.reload();
    } catch (error) {
      console.error(error);
      showErrorToast(error.message);
    }
  });
}

export function initRepoIssueBranchSelect() {
  document.querySelector<HTMLElement>('#branch-select')?.addEventListener('click', (e: DOMEvent<MouseEvent>) => {
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

  const initCombo = async (elCombo: HTMLElement) => {
    const $formField = $(elCombo.querySelector('.form-field-real'));
    const dropzoneContainer = elCombo.querySelector<HTMLElement>('.form-field-dropzone');
    const markdownEditor = elCombo.querySelector<HTMLElement>('.combo-markdown-editor');

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
