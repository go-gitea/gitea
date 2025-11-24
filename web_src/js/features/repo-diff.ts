import {initRepoIssueContentHistory} from './repo-issue-content.ts';
import {initDiffFileTree} from './repo-diff-filetree.ts';
import {initDiffCommitSelect} from './repo-diff-commitselect.ts';
import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.ts';
import {initViewedCheckboxListenerFor, initExpandAndCollapseFilesButton} from './pull-view-file.ts';
import {initImageDiff} from './imagediff.ts';
import {showErrorToast} from '../modules/toast.ts';
import {submitEventSubmitter, queryElemSiblings, hideElem, showElem, animateOnce, addDelegatedEventListener, createElementFromHTML, queryElems} from '../utils/dom.ts';
import {POST, GET} from '../modules/fetch.ts';
import {createTippy} from '../modules/tippy.ts';
import {invertFileFolding, setFileFolding} from './file-fold.ts';
import {parseDom, sleep} from '../utils.ts';
import {registerGlobalSelectorFunc} from '../modules/observer.ts';

const diffLineNumberCellSelector = '#diff-file-boxes .code-diff td.lines-num[data-line-num]';
const diffAnchorSuffixRegex = /([LR])(\d+)$/;
const diffHashRangeRegex = /^(diff-[0-9a-f]+)([LR]\d+)(?:-([LR]\d+))?$/i;

type DiffAnchorSide = 'L' | 'R';
type DiffAnchorInfo = {anchor: string, fragment: string, side: DiffAnchorSide, line: number};
type DiffSelectionState = DiffAnchorInfo & {container: HTMLElement};
type DiffSelectionRange = {fragment: string, startSide: DiffAnchorSide, startLine: number, endSide: DiffAnchorSide, endLine: number};

let diffSelectionStart: DiffSelectionState | null = null;

function changeHash(hash: string) {
  if (window.history.pushState) {
    window.history.pushState(null, null, hash);
  } else {
    window.location.hash = hash;
  }
}

function parseDiffAnchor(anchor: string | null): DiffAnchorInfo | null {
  if (!anchor || !anchor.startsWith('diff-')) return null;
  const suffixMatch = diffAnchorSuffixRegex.exec(anchor);
  if (!suffixMatch) return null;
  const line = Number.parseInt(suffixMatch[2]);
  if (Number.isNaN(line)) return null;
  const fragment = anchor.slice(0, -suffixMatch[0].length);
  const side = suffixMatch[1] as DiffAnchorSide;
  return {anchor, fragment, side, line};
}

function applyDiffLineSelection(container: HTMLElement, range: DiffSelectionRange, options?: {updateHash?: boolean}): boolean {
  // Find the start and end anchor elements
  const startId = `${range.fragment}${range.startSide}${range.startLine}`;
  const endId = `${range.fragment}${range.endSide}${range.endLine}`;
  const startSpan = container.querySelector<HTMLElement>(`#${CSS.escape(startId)}`);
  const endSpan = container.querySelector<HTMLElement>(`#${CSS.escape(endId)}`);

  if (!startSpan || !endSpan) return false;

  const startTr = startSpan.closest('tr');
  const endTr = endSpan.closest('tr');
  if (!startTr || !endTr) return false;

  // Clear previous selection
  for (const tr of document.querySelectorAll('.code-diff tr.active')) {
    tr.classList.remove('active');
  }

  // Get all rows in the diff section
  const allRows = Array.from(container.querySelectorAll<HTMLElement>('.code-diff tbody tr'));
  const startIndex = allRows.indexOf(startTr);
  const endIndex = allRows.indexOf(endTr);

  if (startIndex === -1 || endIndex === -1) return false;

  // Select all rows between start and end (inclusive)
  const minIndex = Math.min(startIndex, endIndex);
  const maxIndex = Math.max(startIndex, endIndex);

  for (let i = minIndex; i <= maxIndex; i++) {
    const row = allRows[i];
    // Only select rows that are actual diff lines (not comment rows, etc.)
    if (row.querySelector('td.lines-num')) {
      row.classList.add('active');
    }
  }

  if (options?.updateHash !== false) {
    const startAnchor = `${range.fragment}${range.startSide}${range.startLine}`;
    const hashValue = (range.startSide === range.endSide && range.startLine === range.endLine) ?
      startAnchor :
      `${startAnchor}-${range.endSide}${range.endLine}`;
    changeHash(`#${hashValue}`);
  }
  return true;
}

function parseDiffHashRange(hashValue: string): DiffSelectionRange | null {
  if (!hashValue.startsWith('diff-')) return null;
  const match = diffHashRangeRegex.exec(hashValue);
  if (!match) return null;
  const startInfo = parseDiffAnchor(`${match[1]}${match[2]}`);
  if (!startInfo) return null;
  let endSide = startInfo.side;
  let endLine = startInfo.line;
  if (match[3]) {
    const endInfo = parseDiffAnchor(`${match[1]}${match[3]}`);
    if (!endInfo) {
      return {fragment: startInfo.fragment, startSide: startInfo.side, startLine: startInfo.line, endSide: startInfo.side, endLine: startInfo.line};
    }
    endSide = endInfo.side;
    endLine = endInfo.line;
  }
  return {
    fragment: startInfo.fragment,
    startSide: startInfo.side,
    startLine: startInfo.line,
    endSide,
    endLine,
  };
}

async function highlightDiffSelectionFromHash(): Promise<boolean> {
  const {hash} = window.location;
  if (!hash || !hash.startsWith('#diff-')) return false;
  const range = parseDiffHashRange(hash.substring(1));
  if (!range) return false;
  const targetId = `${range.fragment}${range.startSide}${range.startLine}`;

  // Wait for the target element to be available (in case it needs to be loaded)
  const targetSpan = document.querySelector<HTMLElement>(`#${CSS.escape(targetId)}`);
  if (!targetSpan) {
    // Target not found - it might need to be loaded via "show more files"
    // Return false to let onLocationHashChange handle the loading
    return false;
  }

  const container = targetSpan.closest<HTMLElement>('.diff-file-box');
  if (!container) return false;

  // Check if the file is collapsed and expand it if needed
  if (container.getAttribute('data-folded') === 'true') {
    const foldBtn = container.querySelector<HTMLElement>('.fold-file');
    if (foldBtn) {
      // Expand the file using the setFileFolding utility
      setFileFolding(container, foldBtn, false);
      // Wait a bit for the expansion animation
      await sleep(100);
    }
  }

  if (!applyDiffLineSelection(container, range, {updateHash: false})) return false;
  diffSelectionStart = {
    anchor: targetId,
    fragment: range.fragment,
    side: range.startSide,
    line: range.startLine,
    container,
  };

  // Scroll to the first selected line (scroll to the tr element, not the span)
  // The span is an inline element inside td, we need to scroll to the tr for better visibility
  await sleep(10);
  const targetTr = targetSpan.closest('tr');
  if (targetTr) {
    targetTr.scrollIntoView({behavior: 'smooth', block: 'center'});
  }
  return true;
}

function handleDiffLineNumberClick(cell: HTMLElement, e: MouseEvent) {
  let span = cell.querySelector<HTMLSpanElement>('span[id^="diff-"]');
  let info = parseDiffAnchor(span?.id ?? null);

  // If clicked cell has no line number (e.g., clicking on the empty side of a deletion/addition),
  // try to find the line number from the sibling cell on the same row
  if (!info) {
    const row = cell.closest('tr');
    if (!row) return;
    // Find the other line number cell in the same row
    const siblingCell = cell.classList.contains('lines-num-old') ?
      row.querySelector<HTMLElement>('td.lines-num-new') :
      row.querySelector<HTMLElement>('td.lines-num-old');
    if (siblingCell) {
      span = siblingCell.querySelector<HTMLSpanElement>('span[id^="diff-"]');
      info = parseDiffAnchor(span?.id ?? null);
    }
    if (!info) return;
  }

  const container = cell.closest<HTMLElement>('.diff-file-box');
  if (!container) return;

  let rangeStart: DiffAnchorInfo = info;
  if (e.shiftKey && diffSelectionStart &&
    diffSelectionStart.container === container &&
    diffSelectionStart.fragment === info.fragment) {
    rangeStart = diffSelectionStart;
  }

  const range: DiffSelectionRange = {
    fragment: info.fragment,
    startSide: rangeStart.side,
    startLine: rangeStart.line,
    endSide: info.side,
    endLine: info.line,
  };

  if (applyDiffLineSelection(container, range)) {
    diffSelectionStart = {...info, container};
    window.getSelection().removeAllRanges();
  }
}

function initDiffLineSelection() {
  addDelegatedEventListener<HTMLElement, MouseEvent>(document, 'click', diffLineNumberCellSelector, (cell, e) => {
    handleDiffLineNumberClick(cell, e);
  });
  window.addEventListener('hashchange', () => {
    highlightDiffSelectionFromHash();
  });
  highlightDiffSelectionFromHash();
}

function initRepoDiffFileBox(el: HTMLElement) {
  // switch between "rendered" and "source", for image and CSV files
  queryElems(el, '.file-view-toggle', (btn) => btn.addEventListener('click', () => {
    queryElemSiblings(btn, '.file-view-toggle', (el) => el.classList.remove('active'));
    btn.classList.add('active');

    const target = document.querySelector(btn.getAttribute('data-toggle-selector'));
    if (!target) throw new Error('Target element not found');

    hideElem(queryElemSiblings(target));
    showElem(target);
  }));
}

function initRepoDiffConversationForm() {
  // FIXME: there could be various different form in a conversation-holder (for example: reply form, edit form).
  // This listener is for "reply form" only, it should clearly distinguish different forms in the future.
  addDelegatedEventListener<HTMLFormElement, SubmitEvent>(document, 'submit', '.conversation-holder form', async (form, e) => {
    e.preventDefault();
    const textArea = form.querySelector<HTMLTextAreaElement>('textarea');
    if (!validateTextareaNonEmpty(textArea)) return;
    if (form.classList.contains('is-loading')) return;

    try {
      form.classList.add('is-loading');
      const formData = new FormData(form);

      // if the form is submitted by a button, append the button's name and value to the form data
      const submitter = submitEventSubmitter(e);
      const isSubmittedByButton = (submitter?.nodeName === 'BUTTON') || (submitter?.nodeName === 'INPUT' && submitter.type === 'submit');
      if (isSubmittedByButton && submitter.name) {
        formData.append(submitter.name, submitter.value);
      }

      // on the diff page, the form is inside a "tr" and need to get the line-type ahead
      // but on the conversation page, there is no parent "tr"
      const trLineType = form.closest('tr')?.getAttribute('data-line-type');
      const response = await POST(form.getAttribute('action'), {data: formData});
      const newConversationHolder = createElementFromHTML(await response.text());
      const path = newConversationHolder.getAttribute('data-path');
      const side = newConversationHolder.getAttribute('data-side');
      const idx = newConversationHolder.getAttribute('data-idx');

      form.closest('.conversation-holder').replaceWith(newConversationHolder);
      form = null; // prevent further usage of the form because it should have been replaced

      if (trLineType) {
        // if there is a line-type for the "tr", it means the form is on the diff page
        // then hide the "add-code-comment" [+] button for current code line by adding "tw-invisible" because the conversation has been added
        let selector;
        if (trLineType === 'same') {
          selector = `[data-path="${path}"] .add-code-comment[data-idx="${idx}"]`;
        } else {
          selector = `[data-path="${path}"] .add-code-comment[data-side="${side}"][data-idx="${idx}"]`;
        }
        for (const el of document.querySelectorAll(selector)) {
          el.classList.add('tw-invisible');
        }
      }

      // the default behavior is to add a pending review, so if no submitter, it also means "pending_review"
      if (!submitter || submitter?.matches('button[name="pending_review"]')) {
        const reviewBox = document.querySelector('#review-box');
        const counter = reviewBox?.querySelector('.review-comments-counter');
        if (!counter) return;
        const num = parseInt(counter.getAttribute('data-pending-comment-number')) + 1 || 1;
        counter.setAttribute('data-pending-comment-number', String(num));
        counter.textContent = String(num);
        animateOnce(reviewBox, 'pulse-1p5-200');
      }
    } catch (error) {
      console.error('Error:', error);
      showErrorToast(`Submit form failed: ${error}`);
    } finally {
      form?.classList.remove('is-loading');
    }
  });

  addDelegatedEventListener(document, 'click', '.resolve-conversation', async (el, e) => {
    e.preventDefault();
    const comment_id = el.getAttribute('data-comment-id');
    const origin = el.getAttribute('data-origin');
    const action = el.getAttribute('data-action');
    const url = el.getAttribute('data-update-url');

    try {
      const response = await POST(url, {data: new URLSearchParams({origin, action, comment_id})});
      const data = await response.text();

      const elConversationHolder = el.closest('.conversation-holder');
      if (elConversationHolder) {
        const elNewConversation = createElementFromHTML(data);
        elConversationHolder.replaceWith(elNewConversation);
      } else {
        window.location.reload();
      }
    } catch (error) {
      console.error('Error:', error);
    }
  });
}

function initRepoDiffConversationNav() {
  // Previous/Next code review conversation
  addDelegatedEventListener(document, 'click', '.previous-conversation, .next-conversation', (el, e) => {
    e.preventDefault();
    const isPrevious = el.matches('.previous-conversation');
    const elCurConversation = el.closest('.comment-code-cloud');
    const elAllConversations = document.querySelectorAll('.comment-code-cloud:not(.tw-hidden)');
    const index = Array.from(elAllConversations).indexOf(elCurConversation);
    const previousIndex = index > 0 ? index - 1 : elAllConversations.length - 1;
    const nextIndex = index < elAllConversations.length - 1 ? index + 1 : 0;
    const navIndex = isPrevious ? previousIndex : nextIndex;
    const elNavConversation = elAllConversations[navIndex];
    const anchor = elNavConversation.querySelector('.comment').id;
    window.location.href = `#${anchor}`;
  });
}

function initDiffHeaderPopup() {
  for (const btn of document.querySelectorAll('.diff-header-popup-btn:not([data-header-popup-initialized])')) {
    btn.setAttribute('data-header-popup-initialized', '');
    const popup = btn.nextElementSibling;
    if (!popup?.matches('.tippy-target')) throw new Error('Popup element not found');
    createTippy(btn, {
      content: popup,
      theme: 'menu',
      placement: 'bottom-end',
      trigger: 'click',
      interactive: true,
      hideOnClick: true,
    });
  }
}

// Will be called when the show more (files) button has been pressed
async function onShowMoreFiles() {
  // TODO: replace these calls with the "observer.ts" methods
  initRepoIssueContentHistory();
  initViewedCheckboxListenerFor();
  initImageDiff();
  initDiffHeaderPopup();
  // Re-apply hash selection in case the target was just loaded
  await highlightDiffSelectionFromHash();
}

async function loadMoreFiles(btn: Element): Promise<boolean> {
  if (btn.classList.contains('disabled')) {
    return false;
  }

  btn.classList.add('disabled');
  const url = btn.getAttribute('data-href');
  try {
    const response = await GET(url);
    const resp = await response.text();
    const respDoc = parseDom(resp, 'text/html');
    const respFileBoxes = respDoc.querySelector('#diff-file-boxes');
    // the response is a full HTML page, we need to extract the relevant contents:
    // * append the newly loaded file list items to the existing list
    document.querySelector('#diff-incomplete').replaceWith(...Array.from(respFileBoxes.children));
    onShowMoreFiles();
    return true;
  } catch (error) {
    console.error('Error:', error);
    showErrorToast('An error occurred while loading more files.');
  } finally {
    btn.classList.remove('disabled');
  }
  return false;
}

function initRepoDiffShowMore() {
  addDelegatedEventListener(document, 'click', 'a#diff-show-more-files', (el, e) => {
    e.preventDefault();
    loadMoreFiles(el);
  });

  addDelegatedEventListener(document, 'click', 'a.diff-load-button', async (el, e) => {
    e.preventDefault();
    if (el.classList.contains('disabled')) return;

    el.classList.add('disabled');
    const url = el.getAttribute('data-href');

    try {
      const response = await GET(url);
      const resp = await response.text();
      const respDoc = parseDom(resp, 'text/html');
      const respFileBody = respDoc.querySelector('#diff-file-boxes .diff-file-body .file-body');
      const respFileBodyChildren = Array.from(respFileBody.children); // respFileBody.children will be empty after replaceWith
      el.parentElement.replaceWith(...respFileBodyChildren);
      for (const el of respFileBodyChildren) window.htmx.process(el);
      // FIXME: calling onShowMoreFiles is not quite right here.
      // But since onShowMoreFiles mixes "init diff box" and "init diff body" together,
      // so it still needs to call it to make the "ImageDiff" and something similar work.
      onShowMoreFiles();
    } catch (error) {
      console.error('Error:', error);
    } finally {
      el.classList.remove('disabled');
    }
  });
}

async function onLocationHashChange() {
  // try to scroll to the target element by the current hash
  const currentHash = window.location.hash;
  if (!currentHash.startsWith('#diff-') && !currentHash.startsWith('#issuecomment-')) return;

  // avoid reentrance when we are changing the hash to scroll and trigger ":target" selection
  const attrAutoScrollRunning = 'data-auto-scroll-running';
  if (document.body.hasAttribute(attrAutoScrollRunning)) return;

  // Check if this is a diff line selection hash (e.g., #diff-xxxL10 or #diff-xxxL10-R20)
  const hashValue = currentHash.substring(1);
  const range = parseDiffHashRange(hashValue);
  if (range) {
    // This is a line selection hash, try to highlight it first
    const success = await highlightDiffSelectionFromHash();
    if (success) {
      // Successfully highlighted and scrolled, we're done
      return;
    }
    // If not successful, fall through to load more files
  }

  const targetElementId = hashValue;
  while (currentHash === window.location.hash) {
    // For line selections, check the range-based target
    let targetElement;
    if (range) {
      const targetId = `${range.fragment}${range.startSide}${range.startLine}`;
      // eslint-disable-next-line unicorn/prefer-query-selector
      targetElement = document.getElementById(targetId);
      if (targetElement) {
        // Try again to highlight and scroll now that the element is loaded
        await highlightDiffSelectionFromHash();
        return;
      }
    } else {
      // use getElementById to avoid querySelector throws an error when the hash is invalid
      // eslint-disable-next-line unicorn/prefer-query-selector
      targetElement = document.getElementById(targetElementId);
      if (targetElement) {
        // need to change hash to re-trigger ":target" CSS selector, let's manually scroll to it
        targetElement.scrollIntoView();
        document.body.setAttribute(attrAutoScrollRunning, 'true');
        window.location.hash = '';
        window.location.hash = currentHash;
        setTimeout(() => document.body.removeAttribute(attrAutoScrollRunning), 0);
        return;
      }
    }

    // If looking for a hidden comment, try to expand the section that contains it
    const issueCommentPrefix = '#issuecomment-';
    if (currentHash.startsWith(issueCommentPrefix)) {
      const commentId = currentHash.substring(issueCommentPrefix.length);
      const expandButton = document.querySelector<HTMLElement>(`.code-expander-button[data-hidden-comment-ids*=",${commentId},"]`);
      if (expandButton) {
        // avoid infinite loop, do not re-click the button if already clicked
        const attrAutoLoadClicked = 'data-auto-load-clicked';
        if (expandButton.hasAttribute(attrAutoLoadClicked)) return;
        expandButton.setAttribute(attrAutoLoadClicked, 'true');
        expandButton.click();
        await sleep(500); // Wait for HTMX to load the content. FIXME: need to drop htmx in the future
        continue; // Try again to find the element
      }
    }

    // the button will be refreshed after each "load more", so query it every time
    const showMoreButton = document.querySelector('#diff-show-more-files');
    if (!showMoreButton) {
      return; // nothing more to load
    }

    // Load more files, await ensures we don't block progress
    const ok = await loadMoreFiles(showMoreButton);
    if (!ok) return; // failed to load more files
  }
}

function initRepoDiffHashChangeListener() {
  window.addEventListener('hashchange', onLocationHashChange);
  onLocationHashChange();
}

export function initRepoDiffView() {
  initRepoDiffConversationForm(); // such form appears on the "conversation" page and "diff" page

  if (!document.querySelector('#diff-file-boxes')) return;
  initRepoDiffConversationNav(); // "previous" and "next" buttons only appear on "diff" page
  initDiffFileTree();
  initDiffCommitSelect();
  initRepoDiffShowMore();
  initDiffHeaderPopup();
  initViewedCheckboxListenerFor();
  initExpandAndCollapseFilesButton();
  initDiffLineSelection();
  initRepoDiffHashChangeListener();

  registerGlobalSelectorFunc('#diff-file-boxes .diff-file-box', initRepoDiffFileBox);
  addDelegatedEventListener(document, 'click', '.fold-file', (el) => {
    invertFileFolding(el.closest('.file-content'), el);
  });
}
