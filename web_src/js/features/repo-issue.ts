import {html, htmlEscape} from '../utils/html.ts';
import {createTippy, showTemporaryTooltip} from '../modules/tippy.ts';
import {
  addDelegatedEventListener,
  createElementFromHTML,
  hideElem,
  queryElems,
  showElem,
  toggleElem,
} from '../utils/dom.ts';
import {setFileFolding} from './file-fold.ts';
import {ComboMarkdownEditor, getComboMarkdownEditor, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {replaceTextareaSelection} from './comp/EditorMarkdown.ts';
import {parseIssuePageInfo, toAbsoluteUrl} from '../utils.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast, showInfoToast} from '../modules/toast.ts';
import {initRepoIssueSidebar} from './repo-issue-sidebar.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {ignoreAreYouSure} from '../vendor/jquery.are-you-sure.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

const {appSubUrl, i18n} = window.config;

export function initRepoIssueSidebarDependency() {
  const elDropdown = document.querySelector('#new-dependency-drop-list');
  if (!elDropdown) return;

  const issuePageInfo = parseIssuePageInfo();
  const crossRepoSearch = elDropdown.getAttribute('data-issue-cross-repo-search');
  let issueSearchUrl = `${issuePageInfo.repoLink}/issues/search?q={query}&type=${issuePageInfo.issueDependencySearchType}`;
  if (crossRepoSearch === 'true') {
    issueSearchUrl = `${appSubUrl}/issues/search?q={query}&priority_repo_id=${issuePageInfo.repoId}&type=${issuePageInfo.issueDependencySearchType}`;
  }
  fomanticQuery(elDropdown).dropdown({
    fullTextSearch: true,
    apiSettings: {
      cache: false,
      rawResponse: true,
      url: issueSearchUrl,
      onResponse(response: any) {
        const filteredResponse = {success: true, results: [] as Array<Record<string, any>>};
        const currIssueId = elDropdown.getAttribute('data-issue-id');
        // Parse the response from the api to work with our dropdown
        for (const issue of response) {
          // Don't list current issue in the dependency list.
          if (String(issue.id) === currIssueId) continue;
          filteredResponse.results.push({
            value: issue.id,
            name: html`<div class="gt-ellipsis">#${issue.number} ${issue.title}</div><div class="text small tw-break-anywhere">${issue.repository.full_name}</div>`,
          });
        }
        return filteredResponse;
      },
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

  const excludeLabel = (e: MouseEvent | KeyboardEvent, item: Element) => {
    e.preventDefault();
    e.stopPropagation();
    const labelId = item.getAttribute('data-label-id')!;
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
    if (e.isComposing) return;
    if (e.altKey && e.key === 'Enter') {
      const selectedItem = elDropdown.querySelector('.label-filter-query-item.selected');
      if (selectedItem) excludeLabel(e, selectedItem);
    }
  });
  // no "labels" query parameter means "all issues"
  elDropdown.querySelector('.label-filter-query-default')!.classList.toggle('selected', queryLabels === '');
  // "labels=0" query parameter means "issues without label"
  elDropdown.querySelector('.label-filter-query-not-set')!.classList.toggle('selected', queryLabels === '0');

  // prepare to process "archived" labels
  const elShowArchivedLabel = elDropdown.querySelector('.label-filter-archived-toggle');
  if (!elShowArchivedLabel) return;
  const elShowArchivedInput = elShowArchivedLabel.querySelector<HTMLInputElement>('input')!;
  elShowArchivedInput.checked = showArchivedLabels;
  const archivedLabels = elDropdown.querySelectorAll('.item[data-is-archived]');
  // if no archived labels, hide the toggle and return
  if (!archivedLabels.length) {
    hideElem(elShowArchivedLabel);
    return;
  }

  // show the archived labels if the toggle is checked or the label is selected
  for (const label of archivedLabels) {
    toggleElem(label, showArchivedLabels || selectedLabelIds.has(label.getAttribute('data-label-id')!));
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
  document.addEventListener('click', async (e) => {
    if (!(e.target as HTMLElement).matches('.delete-comment')) return;
    e.preventDefault();

    const deleteButton = e.target as HTMLElement;
    if (window.confirm(deleteButton.getAttribute('data-locale')!)) {
      try {
        const response = await POST(deleteButton.getAttribute('data-url')!);
        if (!response.ok) throw new Error('Failed to delete comment');

        const conversationHolder = deleteButton.closest('.conversation-holder');
        const parentTimelineItem = deleteButton.closest('.timeline-item');
        const parentTimelineGroup = deleteButton.closest('.timeline-item-group');

        // Check if this was a pending comment.
        if (conversationHolder?.querySelector('.pending-label')) {
          const counter = document.querySelector('#review-box .review-comments-counter')!;
          let num = parseInt(counter?.getAttribute('data-pending-comment-number') || '') - 1 || 0;
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
              document.querySelector(`[data-path="${path}"] .add-code-comment[data-idx="${idx}"]`)!.classList.remove('tw-invisible');
            } else {
              document.querySelector(`[data-path="${path}"] .add-code-comment[data-side="${side}"][data-idx="${idx}"]`)!.classList.remove('tw-invisible');
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

export function initRepoIssueCodeCommentCancel() {
  // Cancel inline code comment
  document.addEventListener('click', (e) => {
    if (!(e.target as HTMLElement).matches('.cancel-code-comment')) return;

    const form = (e.target as HTMLElement).closest('form')!;
    if (form?.classList.contains('comment-form')) {
      hideElem(form);
      showElem(form.closest('.comment-code-cloud')!.querySelectorAll('button.comment-form-reply'));
    } else {
      form.closest('.comment-code-cloud')?.remove();
    }
  });
}

export function initRepoPullRequestAllowMaintainerEdit() {
  const wrapper = document.querySelector('#allow-edits-from-maintainers')!;
  if (!wrapper) return;
  const checkbox = wrapper.querySelector<HTMLInputElement>('input[type="checkbox"]')!;
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
      showTemporaryTooltip(wrapper, wrapper.getAttribute('data-prompt-error')!);
    } finally {
      wrapper.classList.remove('is-loading');
    }
  });
}

export function initRepoIssueComments() {
  if (!document.querySelector('.repository.view.issue .timeline')) return;

  document.addEventListener('click', (e: Event) => {
    const urlTarget = document.querySelector(':target');
    if (!urlTarget) return;

    const urlTargetId = urlTarget.id;
    if (!urlTargetId) return;

    if (!/^(issue|pull)(comment)?-\d+$/.test(urlTargetId)) return;

    if (!(e.target as HTMLElement).closest(`#${urlTargetId}`)) {
      // if the user clicks outside the comment, remove the hash from the url
      // use empty hash and state to avoid scrolling
      window.location.hash = ' ';
      window.history.pushState(null, '', ' ');
    }
  });
}

export async function handleReply(el: HTMLElement) {
  const form = el.closest('.comment-code-cloud')!.querySelector('.comment-form')!;
  const textarea = form.querySelector('textarea');

  hideElem(el);
  showElem(form);
  const editor = getComboMarkdownEditor(textarea) ?? await initComboMarkdownEditor(form.querySelector('.combo-markdown-editor')!);
  editor.focus();
  return editor;
}

export function initSuggestionApplyButtons(root: ParentNode = document) {
  for (const comment of root.querySelectorAll<HTMLElement>('.comment[data-apply-suggestion-url]')) {
    const canApplyValue = comment.getAttribute('data-can-apply-suggestion');
    if (canApplyValue === null) throw new Error('Missing data-can-apply-suggestion on comment');
    if (canApplyValue !== 'true') continue;
    const applyUrl = comment.getAttribute('data-apply-suggestion-url');
    if (!applyUrl) throw new Error('Missing data-apply-suggestion-url on comment');
    const commentId = comment.getAttribute('data-comment-id');
    if (!commentId) throw new Error('Missing data-comment-id on comment');
    const buttonLabel = comment.getAttribute('data-apply-suggestion-text');
    if (!buttonLabel) throw new Error('Missing data-apply-suggestion-text on comment');
    const suggestionBlocks = Array.from(comment.querySelectorAll('pre > code.language-suggestion'));
    let index = 0;
    for (const codeEl of suggestionBlocks) {
      if (codeEl.getAttribute('data-suggestion-initialized')) {
        index++;
        continue;
      }
      codeEl.setAttribute('data-suggestion-initialized', 'true');
      const pre = codeEl.parentElement;
      if (!pre) throw new Error('Suggestion code block is missing a parent element');
      pre.classList.add('suggestion-block');
      const actions = createElementFromHTML(html`
        <div class="suggestion-actions">
          <button type="button" class="ui tiny basic button apply-suggestion-button" data-comment-id="${commentId}" data-suggestion-index="${index}" data-apply-url="${applyUrl}">${buttonLabel}</button>
        </div>
      `);
      pre.after(actions);
      index++;
    }
  }
}

export function initRepoPullRequestReview() {
  if (window.location.hash && window.location.hash.startsWith('#issuecomment-')) {
    const commentDiv = document.querySelector(window.location.hash);
    if (commentDiv) {
      // get the name of the parent id
      const groupID = commentDiv.closest('div[id^="code-comments-"]')?.getAttribute('id');
      if (groupID?.startsWith('code-comments-')) {
        const id = groupID.slice(14);
        const ancestorDiffBox = commentDiv.closest<HTMLElement>('.diff-file-box');

        hideElem(`#show-outdated-${id}`);
        showElem(`#code-comments-${id}, #code-preview-${id}, #hide-outdated-${id}`);
        // if the comment box is folded, expand it
        if (ancestorDiffBox?.getAttribute('data-folded') === 'true') {
          setFileFolding(ancestorDiffBox, ancestorDiffBox.querySelector('.fold-file')!, false);
        }
      }
      // set scrollRestoration to 'manual' when there is a hash in url, so that the scroll position will not be remembered after refreshing
      if (window.history.scrollRestoration !== 'manual') window.history.scrollRestoration = 'manual';
      // wait for a while because some elements (eg: image, editor, etc.) may change the viewport's height.
      setTimeout(() => commentDiv.scrollIntoView({block: 'start'}), 100);
    }
  }

  addDelegatedEventListener(document, 'click', '.show-outdated', (el, e) => {
    e.preventDefault();
    const id = el.getAttribute('data-comment');
    hideElem(el);
    showElem(`#code-comments-${id}`);
    showElem(`#code-preview-${id}`);
    showElem(`#hide-outdated-${id}`);
  });

  addDelegatedEventListener(document, 'click', '.hide-outdated', (el, e) => {
    e.preventDefault();
    const id = el.getAttribute('data-comment');
    hideElem(el);
    hideElem(`#code-comments-${id}`);
    hideElem(`#code-preview-${id}`);
    showElem(`#show-outdated-${id}`);
  });

  addDelegatedEventListener(document, 'click', 'button.comment-form-reply', (el, e) => {
    e.preventDefault();
    handleReply(el);
  });

  initSuggestionApplyButtons();

  addDelegatedEventListener(document, 'click', '.apply-suggestion-button', async (el, e) => {
    e.preventDefault();
    if (el.classList.contains('is-loading')) return;
    const url = el.getAttribute('data-apply-url');
    if (!url) throw new Error('Missing data-apply-url on apply suggestion button');
    const index = el.getAttribute('data-suggestion-index');
    if (index === null) throw new Error('Missing data-suggestion-index on apply suggestion button');
    try {
      el.classList.add('is-loading');
      const response = await POST(url, {data: new URLSearchParams({index})});
      const data = await response.json();
      const {message, ok} = data ?? {};
      if (!response.ok || !ok) {
        showErrorToast(message ?? i18n.error_occurred);
        return;
      }
      showInfoToast(message ?? 'Suggestion applied');
      window.location.reload();
    } catch (error) {
      console.error(error);
      showErrorToast(i18n.error_occurred);
    } finally {
      el.classList.remove('is-loading');
    }
  });

  // The following part is only for diff views
  if (!document.querySelector('.repository.pull.diff')) return;

  type DiffSelection = {
    path: string;
    side: 'left' | 'right';
    start: number;
    end: number;
  };

  let diffSelection: DiffSelection | null = null;

  const clearDiffSelection = () => {
    for (const row of document.querySelectorAll('.code-diff tr.active')) {
      row.classList.remove('active');
    }
  };

  const setDiffSelection = (table: HTMLTableElement, path: string, side: 'left' | 'right', start: number, end: number) => {
    clearDiffSelection();
    const rangeStart = Math.min(start, end);
    const rangeEnd = Math.max(start, end);
    const sideClass = side === 'right' ? 'lines-num-new' : 'lines-num-old';
    for (const td of table.querySelectorAll<HTMLTableCellElement>(`td.lines-num.${sideClass}[data-line-num]`)) {
      const lineValue = td.getAttribute('data-line-num')!;
      const lineNum = Number(lineValue);
      if (!lineNum) continue;
      if (lineNum >= rangeStart && lineNum <= rangeEnd) {
        td.closest('tr')?.classList.add('active');
      }
    }
    diffSelection = {path, side, start: rangeStart, end: rangeEnd};
  };

  const getSuggestionLinesFromDiff = (path: string, side: 'left' | 'right', start: number, end: number): string[] => {
    const table = document.querySelector<HTMLTableElement>(`.code-diff table[data-path="${CSS.escape(path)}"]`);
    if (!table) throw new Error('Suggestion selection table not found');
    const sideClass = side === 'right' ? 'lines-num-new' : 'lines-num-old';
    const lines: string[] = [];
    const rangeStart = Math.min(start, end);
    const rangeEnd = Math.max(start, end);
    for (let lineNum = rangeStart; lineNum <= rangeEnd; lineNum++) {
      const td = table.querySelector<HTMLTableCellElement>(`td.lines-num.${sideClass}[data-line-num="${lineNum}"]`);
      if (!td) {
        lines.push('');
        continue;
      }
      const tr = td.closest('tr');
      const codeEl = tr?.querySelector<HTMLTableCellElement>('td.lines-code code');
      lines.push(codeEl?.textContent ?? '');
    }
    return lines;
  };

  const elReviewBtn = document.querySelector('.js-btn-review');
  const elReviewPanel = document.querySelector('.review-box-panel.tippy-target');
  if (elReviewBtn && elReviewPanel) {
    const tippy = createTippy(elReviewBtn, {
      content: elReviewPanel,
      theme: 'default',
      placement: 'bottom',
      trigger: 'click',
      maxWidth: 'none',
      interactive: true,
      hideOnClick: true,
    });
    elReviewPanel.querySelector('.close')!.addEventListener('click', () => tippy.hide());
  }

  addDelegatedEventListener(document, 'click', '.add-code-comment', async (el, e) => {
    e.preventDefault();

    const isSplit = el.closest('.code-diff')?.classList.contains('code-diff-split');
    const side = el.getAttribute('data-side')!;
    const idx = el.getAttribute('data-idx')!;
    const path = el.closest('[data-path]')?.getAttribute('data-path');
    const tr = el.closest('tr')!;
    const lineType = tr.getAttribute('data-line-type')!;

    let ntr = tr.nextElementSibling;
    if (!ntr?.classList.contains('add-comment')) {
      ntr = createElementFromHTML(`
        <tr class="add-comment" data-line-type="${htmlEscape(lineType)}">
          ${isSplit ? `
            <td class="add-comment-left" colspan="4"></td>
            <td class="add-comment-right" colspan="4"></td>
          ` : `
            <td class="add-comment-left add-comment-right" colspan="5"></td>
          `}
        </tr>`);
      tr.after(ntr);
    }
    const td = ntr.querySelector(`.add-comment-${side}`)!;
    const commentCloud = td.querySelector('.comment-code-cloud');
    if (!commentCloud && !ntr.querySelector('button[name="pending_review"]')) {
      const response = await GET(el.closest('[data-new-comment-url]')?.getAttribute('data-new-comment-url') ?? '');
      td.innerHTML = await response.text();
      const lineStartInput = td.querySelector<HTMLInputElement>("input[name='line_start']")!;
      const lineEndInput = td.querySelector<HTMLInputElement>("input[name='line_end']")!;
      const lineInput = td.querySelector<HTMLInputElement>("input[name='line']")!;
      let lineStart = Number(idx);
      let lineEnd = Number(idx);
      if (diffSelection && diffSelection.path === path && diffSelection.side === side) {
        const {start, end} = diffSelection;
        const lineNum = Number(idx);
        if (lineNum >= start && lineNum <= end) {
          lineStart = start;
          lineEnd = end;
        }
      }
      lineInput.value = String(lineStart);
      lineStartInput.value = String(lineStart);
      lineEndInput.value = String(lineEnd);
      td.querySelector<HTMLInputElement>("input[name='side']")!.value = (side === 'left' ? 'previous' : 'proposed');
      td.querySelector<HTMLInputElement>("input[name='path']")!.value = String(path);
      const editor = await initComboMarkdownEditor(td.querySelector<HTMLElement>('.combo-markdown-editor')!);
      editor.focus();
    }
  });

  addDelegatedEventListener(document, 'click', '.code-diff td.lines-num[data-line-num]:not([data-line-num=""]) span', (el, e: MouseEvent) => {
    const td = el.closest<HTMLTableCellElement>('td.lines-num')!;
    const lineNum = Number(td.getAttribute('data-line-num')!);
    const table = td.closest<HTMLTableElement>('table[data-path]')!;
    const path = table.getAttribute('data-path')!;
    const side = td.classList.contains('lines-num-new') ? 'right' : 'left';
    const isShiftSelect = e.shiftKey && diffSelection?.path === path && diffSelection?.side === side;
    const start = isShiftSelect ? diffSelection!.start : lineNum;
    const end = lineNum;
    setDiffSelection(table, path, side, start, end);
    window.getSelection()?.removeAllRanges();
  });

  addDelegatedEventListener(document, 'click', '.markdown-button-suggestion', (el, e) => {
    e.preventDefault();
    const emptyMessage = el.getAttribute('data-suggestion-empty');
    if (!emptyMessage) throw new Error('Missing data-suggestion-empty on suggestion button');
    const proposedOnlyMessage = el.getAttribute('data-suggestion-proposed-only');
    if (!proposedOnlyMessage) throw new Error('Missing data-suggestion-proposed-only on suggestion button');
    const form = el.closest<HTMLFormElement>('form');
    if (!form) throw new Error('Suggestion button is not inside a form');
    const textarea = form.querySelector<HTMLTextAreaElement>('textarea.markdown-text-editor');
    if (!textarea) throw new Error('Suggestion form is missing the markdown textarea');
    const path = form.querySelector<HTMLInputElement>("input[name='path']")?.value;
    const side = form.querySelector<HTMLInputElement>("input[name='side']")?.value;
    const lineStartValue = form.querySelector<HTMLInputElement>("input[name='line_start']")?.value;
    const lineEndValue = form.querySelector<HTMLInputElement>("input[name='line_end']")?.value;
    if (!path) throw new Error('Suggestion form missing path');
    if (!side) throw new Error('Suggestion form missing side');
    if (!lineStartValue || !lineEndValue) throw new Error('Suggestion form missing line range');
    if (side !== 'proposed') {
      showErrorToast(proposedOnlyMessage);
      return;
    }
    const lineStart = Number(lineStartValue);
    const lineEnd = Number(lineEndValue);
    if (!Number.isInteger(lineStart) || !Number.isInteger(lineEnd) || lineStart <= 0 || lineEnd <= 0) {
      throw new Error('Suggestion form has invalid line range');
    }
    const lines = getSuggestionLinesFromDiff(path, 'right', lineStart, lineEnd);
    if (!lines.length) {
      showErrorToast(emptyMessage);
      return;
    }
    const suggestionBody = lines.join('\n');
    const block = `\n\`\`\`suggestion\n${suggestionBody}\n\`\`\`\n`;
    replaceTextareaSelection(textarea, block);
    textarea.focus();
  });
}

export function initRepoIssueReferenceIssue() {
  const elDropdown = document.querySelector('.issue_reference_repository_search');
  if (!elDropdown) return;
  const form = elDropdown.closest('form')!;
  fomanticQuery(elDropdown).dropdown({
    fullTextSearch: true,
    apiSettings: {
      cache: false,
      rawResponse: true,
      url: `${appSubUrl}/repo/search?q={query}&limit=20`,
      onResponse(response: any) {
        const filteredResponse = {success: true, results: [] as Array<Record<string, any>>};
        for (const repo of response.data) {
          filteredResponse.results.push({
            name: htmlEscape(repo.repository.full_name),
            value: repo.repository.full_name,
          });
        }
        return filteredResponse;
      },
    },
    onChange(_value: string, _text: string, _$choice: any) {
      form.setAttribute('action', `${appSubUrl}/${_text}/issues/new`);
    },
  });

  // Reference issue
  addDelegatedEventListener(document, 'click', '.reference-issue', (el, e) => {
    e.preventDefault();
    const target = el.getAttribute('data-target');
    const content = document.querySelector(`#${target}`)?.textContent ?? '';
    const poster = el.getAttribute('data-poster-username');
    const reference = toAbsoluteUrl(el.getAttribute('data-reference')!);
    const modalSelector = el.getAttribute('data-modal')!;
    const modal = document.querySelector(modalSelector)!;
    const textarea = modal.querySelector<HTMLTextAreaElement>('textarea[name="content"]')!;
    textarea.value = `${content}\n\n_Originally posted by @${poster} in ${reference}_`;
    fomanticQuery(modal).modal('show');
  });
}

export function initRepoIssueWipNewTitle() {
  // Toggle WIP for new PR
  queryElems(document, '.title_wip_desc > a', (el) => el.addEventListener('click', (e) => {
    e.preventDefault();
    const wipPrefixes = JSON.parse(el.closest('.title_wip_desc')!.getAttribute('data-wip-prefixes')!);
    const titleInput = document.querySelector<HTMLInputElement>('#issue_title')!;
    const titleValue = titleInput.value;
    for (const prefix of wipPrefixes) {
      if (titleValue.startsWith(prefix.toUpperCase())) {
        return;
      }
    }
    titleInput.value = `${wipPrefixes[0]} ${titleValue}`;
  }));
}

export function initRepoIssueWipToggle() {
  // Toggle WIP for existing PR
  registerGlobalInitFunc('initPullRequestWipToggle', (toggleWip) => toggleWip.addEventListener('click', async (e) => {
    e.preventDefault();
    const title = toggleWip.getAttribute('data-title');
    const wipPrefix = toggleWip.getAttribute('data-wip-prefix')!;
    const updateUrl = toggleWip.getAttribute('data-update-url')!;

    const params = new URLSearchParams();
    params.append('title', title?.startsWith(wipPrefix) ? title.slice(wipPrefix.length).trim() : `${wipPrefix.trim()} ${title}`);
    const response = await POST(updateUrl, {data: params});
    if (!response.ok) {
      showErrorToast(`Failed to toggle 'work in progress' status`);
      return;
    }
    window.location.reload();
  }));
}

export function initRepoIssueTitleEdit() {
  const issueTitleDisplay = document.querySelector('#issue-title-display')!;
  const issueTitleEditor = document.querySelector<HTMLFormElement>('#issue-title-editor');
  if (!issueTitleEditor) return;

  const issueTitleInput = issueTitleEditor.querySelector('input')!;
  const oldTitle = issueTitleInput.getAttribute('data-old-title')!;
  issueTitleDisplay.querySelector('#issue-title-edit-show')!.addEventListener('click', () => {
    hideElem(issueTitleDisplay);
    hideElem('#pull-desc-display');
    showElem(issueTitleEditor);
    showElem('#pull-desc-editor');
    if (!issueTitleInput.value.trim()) {
      issueTitleInput.value = oldTitle;
    }
    issueTitleInput.focus();
  });
  issueTitleEditor.querySelector('.ui.cancel.button')!.addEventListener('click', () => {
    hideElem(issueTitleEditor);
    hideElem('#pull-desc-editor');
    showElem(issueTitleDisplay);
    showElem('#pull-desc-display');
  });

  const pullDescEditor = document.querySelector('#pull-desc-editor'); // it may not exist for a merged PR
  const prTargetUpdateUrl = pullDescEditor?.getAttribute('data-target-update-url');

  const editSaveButton = issueTitleEditor.querySelector('.ui.primary.button')!;
  issueTitleEditor.addEventListener('submit', async (e) => {
    e.preventDefault();
    const newTitle = issueTitleInput.value.trim();
    try {
      if (newTitle && newTitle !== oldTitle) {
        const resp = await POST(editSaveButton.getAttribute('data-update-url')!, {data: new URLSearchParams({title: newTitle})});
        if (!resp.ok) {
          throw new Error(`Failed to update issue title: ${resp.statusText}`);
        }
      }
      if (prTargetUpdateUrl) {
        const newTargetBranch = document.querySelector('#pull-target-branch')!.getAttribute('data-branch');
        const oldTargetBranch = document.querySelector('#branch_target')!.textContent;
        if (newTargetBranch !== oldTargetBranch) {
          const resp = await POST(prTargetUpdateUrl, {data: new URLSearchParams({target_branch: String(newTargetBranch)})});
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
  document.querySelector<HTMLElement>('#branch-select')?.addEventListener('click', (e: Event) => {
    const el = (e.target as HTMLElement).closest('.item[data-branch]');
    if (!el) return;
    const pullTargetBranch = document.querySelector('#pull-target-branch')!;
    const baseName = pullTargetBranch.getAttribute('data-basename');
    const branchNameNew = el.getAttribute('data-branch')!;
    const branchNameOld = pullTargetBranch.getAttribute('data-branch');
    pullTargetBranch.textContent = pullTargetBranch.textContent.replace(`${baseName}:${branchNameOld}`, `${baseName}:${branchNameNew}`);
    pullTargetBranch.setAttribute('data-branch', branchNameNew);
  });
}

async function initSingleCommentEditor(commentForm: HTMLFormElement) {
  // pages:
  // * normal new issue/pr page: no status-button, no comment-button (there is only a normal submit button which can submit empty content)
  // * issue/pr view page: with comment form, has status-button and comment-button
  const editor = await initComboMarkdownEditor(commentForm.querySelector('.combo-markdown-editor')!);
  const statusButton = document.querySelector<HTMLButtonElement>('#status-button');
  const commentButton = document.querySelector<HTMLButtonElement>('#comment-button');
  const syncUiState = () => {
    const editorText = editor.value().trim(), isUploading = editor.isUploading();
    if (statusButton) {
      const statusText = statusButton.getAttribute(editorText ? 'data-status-and-comment' : 'data-status');
      statusButton.querySelector<HTMLElement>('.status-button-text')!.textContent = statusText;
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

function initIssueTemplateCommentEditors(commentForm: HTMLFormElement) {
  // pages:
  // * new issue with issue template
  const comboFields = commentForm.querySelectorAll<HTMLElement>('.combo-editor-dropzone');

  const initCombo = async (elCombo: HTMLElement) => {
    const fieldTextarea = elCombo.querySelector<HTMLTextAreaElement>('.form-field-real')!;
    const dropzoneContainer = elCombo.querySelector<HTMLElement>('.form-field-dropzone')!;
    const markdownEditor = elCombo.querySelector<HTMLElement>('.combo-markdown-editor')!;

    const editor = await initComboMarkdownEditor(markdownEditor);
    editor.container.addEventListener(ComboMarkdownEditor.EventEditorContentChanged, () => fieldTextarea.value = editor.value());

    fieldTextarea.addEventListener('focus', async () => {
      // deactivate all markdown editors
      showElem(commentForm.querySelectorAll('.combo-editor-dropzone .form-field-real'));
      hideElem(commentForm.querySelectorAll('.combo-editor-dropzone .combo-markdown-editor'));
      queryElems(commentForm, '.combo-editor-dropzone .form-field-dropzone', (dropzoneContainer) => {
        // if "form-field-dropzone" exists, then "dropzone" must also exist
        const dropzone = dropzoneContainer.querySelector<HTMLElement>('.dropzone')!.dropzone;
        const hasUploadedFiles = dropzone.files.length !== 0;
        toggleElem(dropzoneContainer, hasUploadedFiles);
      });

      // activate this markdown editor
      hideElem(fieldTextarea);
      showElem(markdownEditor);
      showElem(dropzoneContainer);

      await editor.switchToUserPreference();
      editor.focus();
    });
  };

  for (const el of comboFields) {
    initCombo(el);
  }
}

export function initRepoCommentFormAndSidebar() {
  const commentForm = document.querySelector<HTMLFormElement>('.comment.form');
  if (!commentForm) return;

  if (commentForm.querySelector('.field.combo-editor-dropzone')) {
    // at the moment, if a form has multiple combo-markdown-editors, it must be an issue template form
    initIssueTemplateCommentEditors(commentForm);
  } else if (commentForm.querySelector('.combo-markdown-editor')) {
    // it's quite unclear about the "comment form" elements, sometimes it's for issue comment, sometimes it's for file editor/uploader message
    initSingleCommentEditor(commentForm);
  }

  initRepoIssueSidebar();
}
