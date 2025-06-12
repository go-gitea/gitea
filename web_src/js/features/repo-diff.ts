import {initRepoIssueContentHistory} from './repo-issue-content.ts';
import {initDiffFileTree} from './repo-diff-filetree.ts';
import {initDiffCommitSelect} from './repo-diff-commitselect.ts';
import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.ts';
import {initViewedCheckboxListenerFor, countAndUpdateViewedFiles, initExpandAndCollapseFilesButton} from './pull-view-file.ts';
import {initImageDiff} from './imagediff.ts';
import {showErrorToast} from '../modules/toast.ts';
import {submitEventSubmitter, queryElemSiblings, hideElem, showElem, animateOnce, addDelegatedEventListener, createElementFromHTML, queryElems} from '../utils/dom.ts';
import {POST, GET} from '../modules/fetch.ts';
import {createTippy} from '../modules/tippy.ts';
import {invertFileFolding} from './file-fold.ts';
import {parseDom} from '../utils.ts';
import {registerGlobalSelectorFunc} from '../modules/observer.ts';

const {i18n} = window.config;

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
      showErrorToast(i18n.network_error);
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
    createTippy(btn, {content: popup, theme: 'menu', placement: 'bottom', trigger: 'click', interactive: true, hideOnClick: true});
  }
}

// Will be called when the show more (files) button has been pressed
function onShowMoreFiles() {
  // TODO: replace these calls with the "observer.ts" methods
  initRepoIssueContentHistory();
  initViewedCheckboxListenerFor();
  countAndUpdateViewedFiles();
  initImageDiff();
  initDiffHeaderPopup();
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

async function loadUntilFound() {
  const hashTargetSelector = window.location.hash;
  if (!hashTargetSelector.startsWith('#diff-') && !hashTargetSelector.startsWith('#issuecomment-')) {
    return;
  }

  while (true) {
    // use getElementById to avoid querySelector throws an error when the hash is invalid
    // eslint-disable-next-line unicorn/prefer-query-selector
    const targetElement = document.getElementById(hashTargetSelector.substring(1));
    if (targetElement) {
      targetElement.scrollIntoView();
      return;
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
  window.addEventListener('hashchange', loadUntilFound);
  loadUntilFound();
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

  registerGlobalSelectorFunc('#diff-file-boxes .diff-file-box', initRepoDiffFileBox);
  addDelegatedEventListener(document, 'click', '.fold-file', (el) => {
    invertFileFolding(el.closest('.file-content'), el);
  });
}
