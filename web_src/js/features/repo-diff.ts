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
import {parseDom} from '../utils.ts';
import {registerGlobalEventFunc, registerGlobalSelectorFunc} from '../modules/observer.ts';
import {svg} from '../svg.ts';

function initRepoDiffFileBox(el: HTMLElement) {
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
      const submitter = submitEventSubmitter(e);
      const isSubmittedByButton = (submitter?.nodeName === 'BUTTON') || (submitter?.nodeName === 'INPUT' && submitter.type === 'submit');
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
      showErrorToast(`Submit form failed: ${error}`);
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
function onShowMoreFiles() {
  // TODO: replace these calls with the "observer.ts" methods
  initRepoIssueContentHistory();
  initViewedCheckboxListenerFor();
  initImageDiff();
  initDiffHeaderPopup();
}

async function loadMoreFiles(btn: Element): Promise<boolean> {
  if (btn.classList.contains('disabled')) {
    return false;
  }

  btn.classList.add('disabled');
  const url = btn.getAttribute('data-href')!;
  try {
    const response = await GET(url);
    const resp = await response.text();
    const respDoc = parseDom(resp, 'text/html');
    const respFileBoxes = respDoc.querySelector('#diff-file-boxes')!;
    // the response is a full HTML page, we need to extract the relevant contents:
    // * append the newly loaded file list items to the existing list
    const respFileBoxesChildren = Array.from(respFileBoxes.children); // "children:HTMLCollection" will be empty after replaceWith
    document.querySelector('#diff-incomplete')!.replaceWith(...respFileBoxesChildren);
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
    const url = el.getAttribute('data-href')!;

    try {
      const response = await GET(url);
      const resp = await response.text();
      const respDoc = parseDom(resp, 'text/html');
      const respFileBody = respDoc.querySelector('#diff-file-boxes .diff-file-body .file-body')!;
      const respFileBodyChildren = Array.from(respFileBody.children); // "children:HTMLCollection" will be empty after replaceWith
      el.parentElement!.replaceWith(...respFileBodyChildren);
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

  const targetElementId = currentHash.substring(1);
  while (currentHash === window.location.hash) {
    // use getElementById to avoid querySelector throws an error when the hash is invalid
    // eslint-disable-next-line unicorn/prefer-query-selector
    const targetElement = document.getElementById(targetElementId);
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
      const expandButton = document.querySelector<HTMLElement>(`.code-expander-button[data-hidden-comment-ids*=",${commentId},"]`);
      if (expandButton) {
        // avoid infinite loop, do not re-expand the same button
        const attrAutoLoadClicked = 'data-auto-load-clicked';
        if (expandButton.hasAttribute(attrAutoLoadClicked)) return;
        expandButton.setAttribute(attrAutoLoadClicked, 'true');
        await fetchBlobExcerpt(expandButton);
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

const expandAllSavedState = new Map<string, string>();

async function fetchBlobExcerpt(btn: HTMLElement) {
  if (btn.classList.contains('is-loading')) return;
  const tr = btn.closest('tr')!;
  const diffBox = tr.closest<HTMLElement>('.diff-file-box')!;
  // save before loading class is added, so restored state is clean
  if (!expandAllSavedState.has(diffBox.id)) {
    const tbody = diffBox.querySelector<HTMLElement>('.code-diff tbody');
    if (tbody) expandAllSavedState.set(diffBox.id, tbody.innerHTML);
  }
  btn.classList.add('is-loading', 'loading-icon-2px');
  try {
    const response = await GET(btn.getAttribute('data-url')!);
    if (!response.ok) return;
    tr.insertAdjacentHTML('afterend', await response.json());
    tr.remove();
    const tbody = diffBox.querySelector('.code-diff tbody');
    if (tbody && !tbody.querySelector('.code-expander-button')) {
      const expandAllBtn = diffBox.querySelector<HTMLElement>('[data-global-click="onDiffExpandAll"]');
      if (expandAllBtn) {
        expandAllBtn.innerHTML = svg('octicon-fold', 14);
        expandAllBtn.setAttribute('data-tooltip-content', expandAllBtn.getAttribute('data-tooltip-collapse')!);
      }
    }
  } finally {
    btn.classList.remove('is-loading', 'loading-icon-2px');
  }
}

function initDiffExpand() {
  const gapParams = ['last_left', 'last_right', 'left', 'right', 'left_hunk_size', 'right_hunk_size'] as const;

  registerGlobalEventFunc('click', 'onExpanderButtonClick', (btn: HTMLElement) => fetchBlobExcerpt(btn));

  registerGlobalEventFunc('click', 'onDiffExpandAll', async (btn: HTMLElement) => {
    if (btn.classList.contains('is-loading')) return;
    const diffBox = btn.closest('.diff-file-box')!;
    const boxId = diffBox.id;
    const tbody = diffBox.querySelector<HTMLElement>('.code-diff tbody');
    // collapse mode: restore saved state when fully expanded (no gaps remain)
    if (tbody && expandAllSavedState.has(boxId) && !tbody.querySelector('.code-expander-button')) {
      tbody.innerHTML = expandAllSavedState.get(boxId)!;
      expandAllSavedState.delete(boxId);
      btn.innerHTML = svg('octicon-unfold', 14);
      btn.setAttribute('data-tooltip-content', btn.getAttribute('data-tooltip-expand')!);
      return;
    }
    if (!tbody) return;
    if (diffBox.getAttribute('data-folded') === 'true') {
      setFileFolding(diffBox, diffBox.querySelector<HTMLElement>('.fold-file')!, false);
    }
    // collect unique TRs with expand buttons (dedup for updown gaps)
    const expandTrs: HTMLTableRowElement[] = [];
    const seen = new Set<HTMLTableRowElement>();
    for (const expandBtn of tbody.querySelectorAll<HTMLElement>('.code-expander-button')) {
      const tr = expandBtn.closest('tr')!;
      if (seen.has(tr)) continue;
      seen.add(tr);
      expandTrs.push(tr);
    }
    if (!expandTrs.length) return;
    if (!expandAllSavedState.has(boxId)) expandAllSavedState.set(boxId, tbody.innerHTML);
    const baseUrl = new URL(expandTrs[0].querySelector<HTMLElement>('.code-expander-button')!.getAttribute('data-url')!, window.location.origin);
    const batchUrl = new URL(baseUrl.pathname, window.location.origin);
    batchUrl.searchParams.set('expand_all', 'true');
    for (const param of ['style', 'path', 'filelang', 'pull_issue_index', 'show_outdated'] as const) {
      const val = baseUrl.searchParams.get(param);
      if (val) batchUrl.searchParams.set(param, val);
    }
    for (const tr of expandTrs) {
      const trUrl = new URL(tr.querySelector<HTMLElement>('.code-expander-button')!.getAttribute('data-url')!, window.location.origin);
      for (const param of gapParams) batchUrl.searchParams.append(param, trUrl.searchParams.get(param)!);
    }
    btn.classList.add('is-loading', 'loading-icon-2px');
    try {
      const response = await GET(batchUrl.toString());
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const htmlStrings: string[] = await response.json();
      for (let idx = 0; idx < expandTrs.length && idx < htmlStrings.length; idx++) {
        expandTrs[idx].insertAdjacentHTML('afterend', htmlStrings[idx]);
        expandTrs[idx].remove();
      }
      btn.innerHTML = svg('octicon-fold', 14);
      btn.setAttribute('data-tooltip-content', btn.getAttribute('data-tooltip-collapse')!);
    } catch {
      expandAllSavedState.delete(boxId);
      showErrorToast('Failed to expand all sections');
    } finally {
      btn.classList.remove('is-loading', 'loading-icon-2px');
    }
  });
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
  initRepoDiffHashChangeListener();
  initDiffExpand();

  registerGlobalSelectorFunc('#diff-file-boxes .diff-file-box', initRepoDiffFileBox);
  addDelegatedEventListener(document, 'click', '.fold-file', (el) => {
    invertFileFolding(el.closest('.file-content')!, el);
  });
}
