import $ from 'jquery';
import {initCompReactionSelector} from './comp/ReactionSelector.ts';
import {initRepoIssueContentHistory} from './repo-issue-content.ts';
import {initDiffFileTree, initDiffFileList} from './repo-diff-filetree.ts';
import {initDiffCommitSelect} from './repo-diff-commitselect.ts';
import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.ts';
import {initViewedCheckboxListenerFor, countAndUpdateViewedFiles, initExpandAndCollapseFilesButton} from './pull-view-file.ts';
import {initImageDiff} from './imagediff.ts';
import {showErrorToast} from '../modules/toast.ts';
import {
  submitEventSubmitter,
  queryElemSiblings,
  hideElem,
  showElem,
  animateOnce,
  addDelegatedEventListener,
  createElementFromHTML,
} from '../utils/dom.ts';
import {POST, GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {createTippy} from '../modules/tippy.ts';
import {invertFileFolding} from './file-fold.ts';

const {pageData, i18n} = window.config;

function initRepoDiffFileViewToggle() {
  $('.file-view-toggle').on('click', function () {
    for (const el of queryElemSiblings(this)) {
      el.classList.remove('active');
    }
    this.classList.add('active');

    const target = document.querySelector(this.getAttribute('data-toggle-selector'));
    if (!target) return;

    hideElem(queryElemSiblings(target));
    showElem(target);
  });
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
      fomanticQuery(newConversationHolder.querySelectorAll('.ui.dropdown')).dropdown();

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

  $(document).on('click', '.resolve-conversation', async function (e) {
    e.preventDefault();
    const comment_id = $(this).data('comment-id');
    const origin = $(this).data('origin');
    const action = $(this).data('action');
    const url = $(this).data('update-url');

    try {
      const response = await POST(url, {data: new URLSearchParams({origin, action, comment_id})});
      const data = await response.text();

      if ($(this).closest('.conversation-holder').length) {
        const $conversation = $(data);
        $(this).closest('.conversation-holder').replaceWith($conversation);
        $conversation.find('.dropdown').dropdown();
        initCompReactionSelector($conversation[0]);
      } else {
        window.location.reload();
      }
    } catch (error) {
      console.error('Error:', error);
    }
  });
}

export function initRepoDiffConversationNav() {
  // Previous/Next code review conversation
  $(document).on('click', '.previous-conversation', (e) => {
    const $conversation = $(e.currentTarget).closest('.comment-code-cloud');
    const $conversations = $('.comment-code-cloud:not(.tw-hidden)');
    const index = $conversations.index($conversation);
    const previousIndex = index > 0 ? index - 1 : $conversations.length - 1;
    const $previousConversation = $conversations.eq(previousIndex);
    const anchor = $previousConversation.find('.comment').first()[0].getAttribute('id');
    window.location.href = `#${anchor}`;
  });
  $(document).on('click', '.next-conversation', (e) => {
    const $conversation = $(e.currentTarget).closest('.comment-code-cloud');
    const $conversations = $('.comment-code-cloud:not(.tw-hidden)');
    const index = $conversations.index($conversation);
    const nextIndex = index < $conversations.length - 1 ? index + 1 : 0;
    const $nextConversation = $conversations.eq(nextIndex);
    const anchor = $nextConversation.find('.comment').first()[0].getAttribute('id');
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
  initRepoIssueContentHistory();
  initViewedCheckboxListenerFor();
  countAndUpdateViewedFiles();
  initImageDiff();
  initDiffHeaderPopup();
}

export async function loadMoreFiles(url) {
  const target = document.querySelector('a#diff-show-more-files');
  if (target?.classList.contains('disabled') || pageData.diffFileInfo.isLoadingNewData) {
    return;
  }

  pageData.diffFileInfo.isLoadingNewData = true;
  target?.classList.add('disabled');

  try {
    const response = await GET(url);
    const resp = await response.text();
    const $resp = $(resp);
    // the response is a full HTML page, we need to extract the relevant contents:
    // 1. append the newly loaded file list items to the existing list
    $('#diff-incomplete').replaceWith($resp.find('#diff-file-boxes').children());
    // 2. re-execute the script to append the newly loaded items to the JS variables to refresh the DiffFileTree
    $('body').append($resp.find('script#diff-data-script'));

    onShowMoreFiles();
  } catch (error) {
    console.error('Error:', error);
    showErrorToast('An error occurred while loading more files.');
  } finally {
    target?.classList.remove('disabled');
    pageData.diffFileInfo.isLoadingNewData = false;
  }
}

function initRepoDiffShowMore() {
  $(document).on('click', 'a#diff-show-more-files', (e) => {
    e.preventDefault();

    const linkLoadMore = e.target.getAttribute('data-href');
    loadMoreFiles(linkLoadMore);
  });

  $(document).on('click', 'a.diff-load-button', async (e) => {
    e.preventDefault();
    const $target = $(e.target);

    if (e.target.classList.contains('disabled')) {
      return;
    }

    e.target.classList.add('disabled');

    const url = $target.data('href');

    try {
      const response = await GET(url);
      const resp = await response.text();

      if (!resp) {
        return;
      }
      $target.parent().replaceWith($(resp).find('#diff-file-boxes .diff-file-body .file-body').children());
      onShowMoreFiles();
    } catch (error) {
      console.error('Error:', error);
    } finally {
      e.target.classList.remove('disabled');
    }
  });
}

export function initRepoDiffView() {
  initRepoDiffConversationForm();
  if (!$('#diff-file-list').length) return;
  initDiffFileTree();
  initDiffFileList();
  initDiffCommitSelect();
  initRepoDiffShowMore();
  initDiffHeaderPopup();
  initRepoDiffFileViewToggle();
  initViewedCheckboxListenerFor();
  initExpandAndCollapseFilesButton();

  addDelegatedEventListener(document, 'click', '.fold-file', (el) => {
    invertFileFolding(el.closest('.file-content'), el);
  });
}
