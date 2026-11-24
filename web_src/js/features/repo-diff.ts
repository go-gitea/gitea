import {createApp} from 'vue';
import DiffFileTree from '../components/DiffFileTree.vue';
import {initRepoIssueContentHistory} from './repo-issue-content.ts';
import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.ts';
import {initExpandAndCollapseFilesButton, initDiffFileViewedForm} from './pull-view-file.ts';
import {showErrorToast} from '../modules/toast.ts';
import {queryElemSiblings, hideElem, showElem, animateOnce, addDelegatedEventListener, createElementFromHTML, queryElems} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {POST, GET} from '../modules/fetch.ts';
import {createTippy} from '../modules/tippy.ts';
import {invertFileFolding, setFileFolding} from './file-fold.ts';
import {parseDom} from '../utils.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import {performFetchActionRequest, performFetchActionTrigger} from '../modules/fetch-action.ts';
import {applyFiltersToFileBoxes, type DiffFileTreeLocale, diffTreeStore, selectDiffTreeFile} from '../modules/diff-file.ts';
import {initImageDiff} from './imagediff.ts';
import DiffCommitSelector from '../components/DiffCommitSelector.vue';
import {registerAsyncHashChangeListener} from '../modules/hashchange.ts';
import {
  findDiffLine, findDiffLineExpander, parseDiffLineHash, repaintDiffRanges, resolveDiffRange, setAddCommentVisibility,
  setDiffLineSelection, type DiffLineHash,
} from './repo-diff-selection.ts';
import {revealReviewComment} from './repo-issue.ts';

function toggleDiffFileView(btn: HTMLElement) {
  queryElemSiblings(btn, '.file-view-toggle', (el) => el.classList.remove('active'));
  btn.classList.add('active');

  const target = document.querySelector(btn.getAttribute('data-toggle-selector')!)!;
  hideElem(queryElemSiblings(target));
  showElem(target);
}

function initDiffFileViewToggle(el: HTMLElement) {
  queryElems(el, '.file-view-toggle', (btn) => btn.addEventListener('click', () => toggleDiffFileView(btn)));
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

      const codeLineRow = form.closest('tr.add-comment')?.previousElementSibling;
      const response = await performFetchActionRequest(form, {method: 'POST', url: form.getAttribute('action')!, data: formData});
      if (!response) return;
      const newConversationHolder = createElementFromHTML(await response.text());
      const side = newConversationHolder.getAttribute('data-side');

      form.closest('.conversation-holder')!.replaceWith(newConversationHolder);
      (form as any) = null; // prevent further usage of the form because it should have been replaced

      if (codeLineRow) {
        setAddCommentVisibility(codeLineRow, side!, false);
        repaintDiffRanges(codeLineRow);
      }

      // the default behavior is to add a pending review, so if no submitter, it also means "pending_review"
      if (!submitter || submitter?.matches('button[name="pending_review"]')) {
        const reviewBox = document.querySelector('.review-box-button')!;
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
        repaintDiffRanges(elNewConversation);
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

function diffLoadUrl(btn: Element) {
  const url = new URL(btn.getAttribute('data-href')!, window.location.href);
  const loadParams = new Set(url.searchParams.keys());
  for (const [name, value] of new URLSearchParams(window.location.search)) {
    if (!loadParams.has(name)) url.searchParams.append(name, value);
  }
  return url.href;
}

let loadingMoreFiles: Promise<boolean> | undefined;

async function diffLoadMoreFiles(btn: Element) {
  if (loadingMoreFiles) return loadingMoreFiles;
  loadingMoreFiles = loadMoreDiffFiles(btn);
  try {
    return await loadingMoreFiles;
  } finally {
    loadingMoreFiles = undefined;
  }
}

async function loadMoreDiffFiles(btn: Element): Promise<boolean> {
  btn.classList.add('disabled');
  try {
    const resp = await GET(diffLoadUrl(btn));
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
  try {
    const resp = await GET(diffLoadUrl(el));
    if (!resp.ok) return;
    const respText = await resp.text();
    const respDoc = parseDom(respText, 'text/html');
    const respFileBody = respDoc.querySelector('#diff-file-boxes .diff-file-body .file-body')!;
    const respFileBodyChildren = Array.from(respFileBody.children); // "children:HTMLCollection" will be empty after replaceWith
    el.parentElement!.replaceWith(...respFileBodyChildren);
    onDiffFileBodyChange();
  } catch (error) {
    console.error('Error:', error);
  } finally {
    el.classList.remove('disabled');
  }
}

const loadingExcerpts = new WeakMap<Element, Promise<void>>();

async function expandExcerpt(btn: HTMLElement) {
  const row = btn.closest('tr')!;
  const pending = loadingExcerpts.get(row);
  if (pending) return pending;
  const load = performFetchActionTrigger(btn, 'load');
  loadingExcerpts.set(row, load);
  try {
    await load;
  } finally {
    loadingExcerpts.delete(row);
  }
}

async function scrollToDiffLines(box: HTMLElement, range: DiffLineHash, currentHash: string) {
  const loadBodyButton = box.querySelector('[data-global-click="diffLoadFileBody"]');
  if (loadBodyButton) await diffLoadFileBody(loadBodyButton);
  if (currentHash !== window.location.hash) return;

  const table = box.querySelector<HTMLTableElement>('table[data-global-init="initDiffSelection"]');
  if (!table) return;
  const {fileHash, start, end} = range;
  for (const line of new Set([start, end])) {
    while (currentHash === window.location.hash && !findDiffLine(table, fileHash, line)) {
      const button = findDiffLineExpander(table, line);
      if (!button) return;
      await expandExcerpt(button);
      if (button.isConnected) return;
    }
  }
  if (currentHash !== window.location.hash) return;

  const resolved = resolveDiffRange(table, range)!;
  showElem(box);
  const source = table.closest('.code-diff')!;
  if (source.classList.contains('tw-hidden')) {
    toggleDiffFileView(box.querySelector(`.file-view-toggle[data-toggle-selector="#${CSS.escape(source.id)}"]`)!);
  }
  setDiffLineSelection(table, resolved);
  window.history.scrollRestoration = 'manual';
  findDiffLine(table, fileHash, resolved.start.line)!.focus({preventScroll: true});
  resolved.start.row.scrollIntoView({block: 'start'});
}

async function onLocationHashChange() {
  setDiffLineSelection();
  const currentHash = window.location.hash;
  const lineRange = parseDiffLineHash(currentHash);
  const fileHash = lineRange?.fileHash ?? /^#diff-([0-9a-f]{40})$/.exec(currentHash)?.[1];
  const comment = /^#issuecomment-(\d+)$/.exec(currentHash);
  selectDiffTreeFile(fileHash ?? '');
  if (!fileHash && !comment) return;

  const targetId = fileHash ? `diff-${fileHash}` : currentHash.slice(1);
  while (currentHash === window.location.hash) {
    const target = document.querySelector<HTMLElement>(`#${CSS.escape(targetId)}`);
    if (target) {
      if (fileHash && target.getAttribute('data-folded') === 'true') setFileFolding(target, target.querySelector('.fold-file')!, false);
      if (lineRange) {
        await scrollToDiffLines(target, lineRange, currentHash);
      } else {
        if (comment) revealReviewComment(target);
        target.classList.add('diff-hash-target');
        target.scrollIntoView({block: 'start'});
      }
      return;
    }

    if (comment) {
      const button = document.querySelector<HTMLElement>(`.code-expander-button[data-hidden-comment-ids*=",${CSS.escape(comment[1])},"]`);
      if (button) {
        await expandExcerpt(button);
        if (button.isConnected) return;
        continue;
      }
    }
    const showMoreButton = document.querySelector('#diff-show-more-files');
    if (!showMoreButton || !await diffLoadMoreFiles(showMoreButton)) return;
  }
}

function initDiffFileTree(el: HTMLElement) {
  const locale = JSON.parse(el.getAttribute('data-locale')!) as DiffFileTreeLocale;
  createApp(DiffFileTree, {locale}).mount(el);
}

function initDiffCommitSelect(el: HTMLElement) {
  createApp(DiffCommitSelector).mount(el);
}

export function initRepoDiffView() {
  initRepoDiffConversationForm(); // such form appears on the "conversation" page and "diff" page
  registerGlobalEventFunc('click', 'diffLoadMoreFiles', (el) => { diffLoadMoreFiles(el) });
  registerGlobalEventFunc('click', 'diffLoadFileBody', diffLoadFileBody);
  registerGlobalEventFunc('click', 'diffFileViewFold', (el) => invertFileFolding(el.closest('.file-content')!, el));
  registerGlobalInitFunc('initDiffHeaderPopupMenu', initDiffHeaderPopupMenu);
  registerGlobalInitFunc('initDiffFileViewedForm', initDiffFileViewedForm);
  registerGlobalInitFunc('initDiffFileImageDiff', initImageDiff);
  registerGlobalInitFunc('initDiffFileViewToggle', initDiffFileViewToggle);
  registerGlobalInitFunc('initDiffFileTree', initDiffFileTree);
  registerGlobalInitFunc('initDiffCommitSelect', initDiffCommitSelect);

  if (!document.querySelector('#diff-file-boxes')) return;
  initRepoDiffConversationNav(); // "previous" and "next" buttons only appear on "diff" page
  initExpandAndCollapseFilesButton();

  registerAsyncHashChangeListener(onLocationHashChange, 999);
}
