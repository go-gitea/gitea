import {initRepoIssueContentHistory} from './repo-issue-content.ts';
import {initDiffFileTree} from './repo-diff-filetree.ts';
import {initDiffCommitSelect} from './repo-diff-commitselect.ts';
import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.ts';
import {initExpandAndCollapseFilesButton, initDiffFileViewedForm} from './pull-view-file.ts';
import {showErrorToast} from '../modules/toast.ts';
import {queryElemSiblings, hideElem, showElem, animateOnce, addDelegatedEventListener, createElementFromHTML, queryElems, isElemVisible} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {POST, GET} from '../modules/fetch.ts';
import {createTippy} from '../modules/tippy.ts';
import {invertFileFolding, setFileFolding} from './file-fold.ts';
import {parseDom} from '../utils.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import {performFetchActionTrigger, performFetchAction} from './common-fetch-action.ts';
import {initImageDiff} from './imagediff.ts';
import {svg} from '../svg.ts';
import {confirmModal} from './comp/ConfirmModal.ts';

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

// the button is rendered with "tw-hidden", so only show it when the file box actually has an expandable gap
function initDiffExpandAllLinesButton(btn: HTMLElement) {
  const fileBox = btn.closest('.diff-file-box')!;
  if (fileBox.querySelector('.code-expander-buttons[data-expand-all-url]')) showElem(btn);
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

async function fetchDiffFileBodyChildren(url: string): Promise<Element[]> {
  const response = await GET(url);
  if (!response.ok) throw new Error(`Unable to load diff content: ${response.status} ${response.statusText}`);
  const respText = await response.text();
  const respDoc = parseDom(respText, 'text/html');
  const respFileBody = respDoc.querySelector('#diff-file-boxes .diff-file-body .file-body');
  if (!respFileBody) throw new Error('Unable to find diff file body in the response');
  return Array.from(respFileBody.children); // "children:HTMLCollection" will be empty after replaceWith
}

async function diffLoadMoreFiles(btn: Element): Promise<boolean> {
  if (btn.classList.contains('disabled')) return false;
  btn.classList.add('disabled');
  const url = btn.getAttribute('data-href')!;
  try {
    const resp = await GET(url);
    if (!resp.ok) return false;
    const respText = await resp.text();
    const respDoc = parseDom(respText, 'text/html');
    const respFileBoxes = respDoc.querySelector('#diff-file-boxes')!;
    // the response is a full HTML page, we need to extract the relevant contents:
    // * append the newly loaded file list items to the existing list
    const respFileBoxesChildren = Array.from(respFileBoxes.children); // "children:HTMLCollection" will be empty after replaceWith
    document.querySelector('#diff-incomplete')!.replaceWith(...respFileBoxesChildren);
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
    const respFileBodyChildren = await fetchDiffFileBodyChildren(el.getAttribute('data-href')!);
    el.parentElement!.replaceWith(...respFileBodyChildren);
    onDiffFileBodyChange();
  } catch (error) {
    console.error('Error:', error);
  } finally {
    el.classList.remove('disabled');
  }
}

// Toggles the tooltip/aria-label/icon of the "expand all lines" button between its two states.
function updateDiffExpandAllLinesButton(btn: HTMLElement, expanded: boolean) {
  btn.setAttribute('data-expanded', String(expanded));
  const tooltip = btn.getAttribute(expanded ? 'data-tooltip-content-expanded' : 'data-tooltip-content-collapsed')!;
  btn.setAttribute('data-tooltip-content', tooltip); // there is a MutationObserver on this attribute to refresh the tippy content
  btn.setAttribute('aria-label', tooltip);
  btn.innerHTML = svg(expanded ? 'octicon-fold' : 'octicon-unfold', 14);
}

async function expandDiffFileAllLines(fileBox: HTMLElement, btn: HTMLElement) {
  // a folded file (vendored/generated, or already "viewed" in a PR) hides its body via CSS, so expanding
  // its lines would otherwise happen invisibly; unfold it first so the result is actually visible
  if (fileBox.getAttribute('data-folded') === 'true') {
    setFileFolding(fileBox, fileBox.querySelector<HTMLElement>('.fold-file')!, false);
  }
  const gapEls = queryElems<HTMLElement>(fileBox, '.code-expander-buttons[data-expand-all-url]');
  // each gap is expanded through the same fetch-action infra as the existing single-gap expander buttons,
  // "performFetchAction" already checks response.ok and shows an error toast, leaving a failed gap's row untouched
  await Promise.all(Array.from(gapEls, (gapEl) => performFetchAction(gapEl, {
    method: 'GET',
    url: gapEl.getAttribute('data-expand-all-url')!,
    loadingIndicator: '',
    successSync: '$closest(tr)',
  })));
  // a gap element only remains in the DOM if its request failed, so only flip the button once every gap succeeded
  if (!fileBox.querySelector('.code-expander-buttons[data-expand-all-url]')) {
    updateDiffExpandAllLinesButton(btn, true);
  }
}

function hasUnsavedDiffComment(fileBox: HTMLElement): boolean {
  return Array.from(fileBox.querySelectorAll<HTMLTextAreaElement>('.markdown-text-editor'))
    .some((textarea) => textarea.value.trim() !== '' && isElemVisible(textarea.closest('form')!));
}

async function collapseDiffFileAllLines(fileBox: HTMLElement, btn: HTMLElement) {
  if (hasUnsavedDiffComment(fileBox) && !await confirmModal({content: btn.getAttribute('data-collapse-confirm-text')!, confirmButtonColor: 'red'})) return;
  try {
    const respFileBodyChildren = await fetchDiffFileBodyChildren(btn.getAttribute('data-collapse-url')!);
    fileBox.querySelector('.diff-file-body .file-body')!.replaceChildren(...respFileBodyChildren);
    // the global selector observer re-inits the freshly inserted per-element pieces on its own, but the
    // page-wide content-history scan is not per-element, so it still needs to be triggered here
    onDiffFileBodyChange();
    updateDiffExpandAllLinesButton(btn, false);
  } catch (error) {
    console.error('Error:', error);
    showErrorToast('An error occurred while collapsing the file.');
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
      const expandButton = document.querySelector<HTMLElement>(`.code-expander-button[data-hidden-comment-ids*=",${CSS.escape(commentId)},"]`);
      if (expandButton) {
        // avoid infinite loop, do not re-click the button if already clicked
        const attrAutoLoadClicked = 'data-auto-load-clicked';
        if (expandButton.hasAttribute(attrAutoLoadClicked)) return;
        expandButton.setAttribute(attrAutoLoadClicked, 'true');
        // trigger the fetch action to load the hidden comments, after loading, it will try to find the target element again
        await performFetchActionTrigger(expandButton, 'load');
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
  registerGlobalEventFunc('click', 'diffFileViewFold', (el) => invertFileFolding(el.closest('.file-content')!, el));
  registerGlobalEventFunc('click', 'onDiffExpandAllLinesClick', async (btn: HTMLElement) => {
    if (btn.classList.contains('is-loading')) return;
    const fileBox = btn.closest<HTMLElement>('.diff-file-box')!;
    btn.classList.add('is-loading');
    try {
      if (btn.getAttribute('data-expanded') === 'true') {
        await collapseDiffFileAllLines(fileBox, btn);
      } else {
        await expandDiffFileAllLines(fileBox, btn);
      }
    } finally {
      btn.classList.remove('is-loading');
    }
  });
  registerGlobalInitFunc('initDiffHeaderPopupMenu', initDiffHeaderPopupMenu);
  registerGlobalInitFunc('initDiffFileViewedForm', initDiffFileViewedForm);
  registerGlobalInitFunc('initDiffFileImageDiff', initImageDiff);
  registerGlobalInitFunc('initDiffFileViewToggle', initDiffFileViewToggle);
  registerGlobalInitFunc('initDiffExpandAllLinesButton', initDiffExpandAllLinesButton);

  if (!document.querySelector('#diff-file-boxes')) return;
  initRepoDiffConversationNav(); // "previous" and "next" buttons only appear on "diff" page
  initDiffFileTree();
  initDiffCommitSelect();
  initExpandAndCollapseFilesButton();

  window.addEventListener('hashchange', onLocationHashChange);
  onLocationHashChange();
}
