import {initRepoIssueContentHistory} from './repo-issue-content.ts';
import {initDiffFileTree} from './repo-diff-filetree.ts';
import {initDiffCommitSelect} from './repo-diff-commitselect.ts';
import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.ts';
import {initExpandAndCollapseFilesButton, initDiffFileViewedForm} from './pull-view-file.ts';
import {showErrorToast} from '../modules/toast.ts';
import {queryElemSiblings, hideElem, showElem, toggleElem, animateOnce, addDelegatedEventListener, createElementFromHTML, queryElems} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {POST, GET} from '../modules/fetch.ts';
import {createTippy, getAttachedTippyInstance} from '../modules/tippy.ts';
import {setFileFolding, invertFileFolding} from './file-fold.ts';
import {parseDom} from '../utils.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import {performFetchActionRequest} from '../modules/fetch-action.ts';
import {applyFiltersToFileBoxes, diffTreeStore} from '../modules/diff-file.ts';
import {initImageDiff} from './imagediff.ts';
import {closeGap, diffFileHasHiddenLines, gapAfterExpanding, gapExcerptUrl, gapsExcerptUrl, getDiffGapState, initDiffGapExpander, pendingDiffGaps, revealGapLines} from './repo-diff-gaps.ts';

function initDiffFileViewToggle(el: HTMLElement) {
  // switch between "rendered" and "source", for image and CSV files
  queryElems(el, '.file-view-toggle', (btn) => btn.addEventListener('click', () => {
    queryElemSiblings(btn, '.file-view-toggle', (el) => el.classList.remove('active'));
    btn.classList.add('active');

    const target = document.querySelector(btn.getAttribute('data-toggle-selector')!);
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
    const textArea = form.querySelector<HTMLTextAreaElement>('textarea')!;
    if (!validateTextareaNonEmpty(textArea)) return;
    if (form.classList.contains('is-loading')) return;

    try {
      form.classList.add('is-loading');
      const formData = new FormData(form);

      // if the form is submitted by a button, append the button's name and value to the form data
      const submitter = e.submitter;
      const isSubmittedByButton = submitter instanceof HTMLButtonElement || (submitter instanceof HTMLInputElement && submitter.type === 'submit');
      if (isSubmittedByButton && submitter.name) {
        formData.append(submitter.name, submitter.value);
      }

      // on the diff page, the form is inside a "tr" and need to get the line-type ahead
      // but on the conversation page, there is no parent "tr"
      const trLineType = form.closest('tr')?.getAttribute('data-line-type');
      const response = await POST(form.getAttribute('action')!, {data: formData});
      const newConversationHolder = createElementFromHTML(await response.text());
      const path = newConversationHolder.getAttribute('data-path');
      const side = newConversationHolder.getAttribute('data-side');
      const idx = newConversationHolder.getAttribute('data-idx');

      form.closest('.conversation-holder')!.replaceWith(newConversationHolder);
      (form as any) = null; // prevent further usage of the form because it should have been replaced

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
        const reviewBox = document.querySelector('#review-box')!;
        const counter = reviewBox?.querySelector('.review-comments-counter');
        if (!counter) return;
        const num = parseInt(counter.getAttribute('data-pending-comment-number')!) + 1 || 1;
        counter.setAttribute('data-pending-comment-number', String(num));
        counter.textContent = String(num);
        animateOnce(reviewBox, 'pulse-1p5-200');
      }
    } catch (error) {
      console.error('Error:', error);
      showErrorToast(`Submit form failed: ${errorMessage(error)}`);
    } finally {
      form?.classList.remove('is-loading');
    }
  });

  addDelegatedEventListener(document, 'click', '.resolve-conversation', async (el, e) => {
    e.preventDefault();
    const comment_id = el.getAttribute('data-comment-id')!;
    const origin = el.getAttribute('data-origin')!;
    const action = el.getAttribute('data-action')!;
    const url = el.getAttribute('data-update-url')!;

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
    const elCurConversation = el.closest('.comment-code-cloud')!;
    const elAllConversations = document.querySelectorAll('.comment-code-cloud:not(.tw-hidden)');
    const index = Array.from(elAllConversations).indexOf(elCurConversation);
    const previousIndex = index > 0 ? index - 1 : elAllConversations.length - 1;
    const nextIndex = index < elAllConversations.length - 1 ? index + 1 : 0;
    const navIndex = isPrevious ? previousIndex : nextIndex;
    const elNavConversation = elAllConversations[navIndex];
    const anchor = elNavConversation.querySelector('.comment')!.id;
    window.location.assign(`#${anchor}`);
  });
}

function initDiffHeaderPopupMenu(btn: HTMLElement) {
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

function onDiffFileBodyChange() {
  initRepoIssueContentHistory(); // it scans the whole page via a fetch, so it doesn't fit the per-element observer pattern
}

function parseTableRows(respText: string): HTMLElement[] {
  const elTemplate = document.createElement('template'); // a template can hold "tr" elements without a table around them
  elTemplate.innerHTML = respText;
  // the response opens with the table's "colgroup", which makes the parser tuck the rows into a
  // "tbody", so take the rows themselves rather than whatever wrapped them
  return Array.from(elTemplate.content.querySelectorAll('tr'));
}

// the response carries each gap's lines in file order, and a line's conversations in the row after it
function diffGroupRowsByGap(respText: string): Map<string, HTMLElement[]> {
  const gapRows = new Map<string, HTMLElement[]>();
  let gapKey = '';
  for (const elRow of parseTableRows(respText)) {
    gapKey = elRow.getAttribute('data-expand-gap') ?? gapKey;
    if (gapKey) gapRows.set(gapKey, [...gapRows.get(gapKey) ?? [], elRow]);
  }
  return gapRows;
}

function diffExpanderOf(elFileBody: Element, gapKey: string): Element | null {
  return elFileBody.querySelector(`.code-expander-buttons[data-gap-key="${CSS.escape(gapKey)}"]`);
}

function diffSyncExpandAllButton(elFileBox: Element) {
  const btn = elFileBox.querySelector<HTMLElement>('.diff-expand-lines-button');
  if (!btn) return;
  const canExpand = diffFileHasHiddenLines(elFileBox.querySelector('.diff-file-body .file-body')!);
  const text = btn.getAttribute(canExpand ? 'data-text-expand' : 'data-text-collapse')!;
  btn.setAttribute('data-tooltip-content', text);
  btn.setAttribute('aria-label', text);
  getAttachedTippyInstance(btn)?.setContent(text); // the tooltip may already have been created by a hover
  toggleElem(btn.querySelector('.svg.octicon-unfold')!, canExpand);
  toggleElem(btn.querySelector('.svg.octicon-fold')!, !canExpand);
}

async function diffExpandHiddenLines(btn: HTMLElement) {
  const elExpander = btn.closest<HTMLElement>('.code-expander-buttons')!;
  const gapKey = elExpander.getAttribute('data-gap-key')!;
  const baseUrl = elExpander.closest('table')!.getAttribute('data-excerpt-url')!;
  const direction = btn.getAttribute('data-gap-direction')!;
  const gap = getDiffGapState(elExpander, gapKey)!.current;

  const resp = await performFetchActionRequest(btn, {method: 'GET', url: gapExcerptUrl(baseUrl, gap, direction), loadingIndicator: '$this'});
  if (!resp) return;
  // the effect puts the rows on screen and re-renders whatever the gap has left
  revealGapLines(elExpander, gapKey, parseTableRows(await resp.text()), gapAfterExpanding(gap, direction));

  const elFileBox = btn.closest('.diff-file-box'); // absent on the pull conversation page
  if (elFileBox) diffSyncExpandAllButton(elFileBox);
  onDiffFileBodyChange();
}

// one request reveals every gap the file still hides, through the same endpoint the arrows use, so
// the server reads the blob instead of recomputing the diff
async function diffFetchAllGapRows(btn: HTMLElement, elFileBody: Element): Promise<Map<string, HTMLElement[]> | null> {
  const baseUrl = elFileBody.querySelector('table')!.getAttribute('data-excerpt-url')!;
  const resp = await performFetchActionRequest(btn, {method: 'GET', url: gapsExcerptUrl(baseUrl, pendingDiffGaps(elFileBody)), loadingIndicator: '$this'});
  return resp ? diffGroupRowsByGap(await resp.text()) : null;
}

async function diffToggleAllHiddenLines(btn: HTMLElement) {
  const elFileBox = btn.closest<HTMLElement>('.diff-file-box')!;
  const elFileBody = elFileBox.querySelector('.diff-file-body .file-body')!;

  if (diffFileHasHiddenLines(elFileBody)) {
    // expanding a folded file (vendored, generated, already viewed) would happen out of sight
    setFileFolding(elFileBox, elFileBox.querySelector<HTMLElement>('.fold-file')!, false);
    const gapRows = await diffFetchAllGapRows(btn, elFileBody); // fetch before touching the store, so an already expanded gap does not flicker
    if (!gapRows) return;
    for (const [gapKey, rows] of gapRows) {
      const elExpander = diffExpanderOf(elFileBody, gapKey);
      const state = elExpander && getDiffGapState(elExpander, gapKey);
      if (!state) continue;
      closeGap(elExpander, gapKey); // drop what it revealed so far, the response carries the whole gap
      revealGapLines(elExpander, gapKey, rows, {...state.original, left: 0, right: 0, leftHunk: 0, rightHunk: 0});
    }
    onDiffFileBodyChange();
  } else {
    for (const el of elFileBody.querySelectorAll('.code-expander-buttons[data-gap-key]')) {
      closeGap(el, el.getAttribute('data-gap-key')!);
    }
  }
  diffSyncExpandAllButton(elFileBox);
}

async function diffLoadMoreFiles(btn: Element): Promise<boolean> {
  if (btn.classList.contains('disabled')) return false;
  btn.classList.add('disabled');
  const url = btn.getAttribute('data-href')!;
  try {
    const resp = await GET(url);
    if (!resp.ok) return false;
    const respText = await resp.text();
    const respDoc = parseDom(respText, 'text/html'); // the response is a full HTML page, extract the new file boxes from it
    const respFileBoxes = respDoc.querySelector('#diff-file-boxes')!;
    const respFileBoxesChildren = Array.from(respFileBoxes.children); // "children:HTMLCollection" will be empty after replaceWith
    document.querySelector('#diff-incomplete')!.replaceWith(...respFileBoxesChildren);
    applyFiltersToFileBoxes(diffTreeStore());
    onDiffFileBodyChange();
    return true;
  } catch (error) {
    console.error('Error:', error);
    showErrorToast('An error occurred while loading more files.');
  } finally {
    btn.classList.remove('disabled');
  }
  return false;
}

async function diffLoadFileBody(el: Element) {
  if (el.classList.contains('disabled')) return;
  el.classList.add('disabled');
  const url = el.getAttribute('data-href')!;
  try {
    const resp = await GET(url);
    if (!resp.ok) return;
    const respText = await resp.text();
    const respDoc = parseDom(respText, 'text/html');
    // replace the whole file box: its header actions (expand lines, unescape) depend on the file body being loaded
    const respFileBox = respDoc.querySelector('#diff-file-boxes .diff-file-box')!;
    el.closest('.diff-file-box')!.replaceWith(respFileBox);
    onDiffFileBodyChange();
  } catch (error) {
    console.error('Error:', error);
  } finally {
    el.classList.remove('disabled');
  }
}

async function onLocationHashChange() {
  // try to scroll to the target element by the current hash
  const currentHash = window.location.hash;
  if (!currentHash.startsWith('#diff-') && !currentHash.startsWith('#issuecomment-')) return;

  // avoid reentrance when we are changing the hash to scroll and trigger ":target" selection
  const attrAutoScrollRunning = 'data-auto-scroll-running';
  if (document.body.hasAttribute(attrAutoScrollRunning)) return;

  const targetElementId = currentHash.substring(1);
  while (currentHash === window.location.hash) {
    const targetElement = document.querySelector<HTMLElement>(`#${CSS.escape(targetElementId)}`);
    if (targetElement) {
      // need to change hash to re-trigger ":target" CSS selector, let's manually scroll to it
      targetElement.scrollIntoView();
      document.body.setAttribute(attrAutoScrollRunning, 'true');
      window.location.hash = '';
      window.location.hash = currentHash;
      setTimeout(() => document.body.removeAttribute(attrAutoScrollRunning), 0);
      return;
    }

    // If looking for a hidden comment, try to expand the section that contains it
    const issueCommentPrefix = '#issuecomment-';
    if (currentHash.startsWith(issueCommentPrefix)) {
      const commentId = currentHash.substring(issueCommentPrefix.length);
      const expandButton = document.querySelector<HTMLElement>(`.code-expander-buttons[data-hidden-comment-ids*=",${CSS.escape(commentId)},"] .code-expander-button`);
      if (expandButton) {
        // avoid infinite loop, do not re-click the button if already clicked
        const attrAutoLoadClicked = 'data-auto-load-clicked';
        if (expandButton.hasAttribute(attrAutoLoadClicked)) return;
        expandButton.setAttribute(attrAutoLoadClicked, 'true');
        // expand the section to load the hidden comments, after loading, it will try to find the target element again
        await diffExpandHiddenLines(expandButton);
        continue; // Try again to find the element
      }
    }

    // the button will be refreshed after each "load more", so query it every time
    const showMoreButton = document.querySelector('#diff-show-more-files');
    if (!showMoreButton) {
      return; // nothing more to load
    }

    // Load more files, await ensures we don't block progress
    const ok = await diffLoadMoreFiles(showMoreButton);
    if (!ok) return; // failed to load more files
  }
}

export function initRepoDiffView() {
  initRepoDiffConversationForm(); // such form appears on the "conversation" page and "diff" page
  registerGlobalEventFunc('click', 'diffLoadMoreFiles', (el) => { diffLoadMoreFiles(el) });
  registerGlobalEventFunc('click', 'diffLoadFileBody', diffLoadFileBody);
  registerGlobalEventFunc('click', 'diffExpandHiddenLines', diffExpandHiddenLines);
  registerGlobalEventFunc('click', 'diffToggleAllHiddenLines', diffToggleAllHiddenLines);
  registerGlobalEventFunc('click', 'diffFileViewFold', (el) => invertFileFolding(el.closest('.file-content')!, el));
  registerGlobalInitFunc('initDiffGapExpander', initDiffGapExpander);
  registerGlobalInitFunc('initDiffHeaderPopupMenu', initDiffHeaderPopupMenu);
  registerGlobalInitFunc('initDiffFileViewedForm', initDiffFileViewedForm);
  registerGlobalInitFunc('initDiffFileImageDiff', initImageDiff);
  registerGlobalInitFunc('initDiffFileViewToggle', initDiffFileViewToggle);

  if (!document.querySelector('#diff-file-boxes')) return;
  initRepoDiffConversationNav(); // "previous" and "next" buttons only appear on "diff" page
  initDiffFileTree();
  initDiffCommitSelect();
  initExpandAndCollapseFilesButton();

  window.addEventListener('hashchange', onLocationHashChange);
  onLocationHashChange();
}
