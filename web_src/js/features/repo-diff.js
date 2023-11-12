import $ from 'jquery';
import {initCompReactionSelector} from './comp/ReactionSelector.js';
import {initRepoIssueContentHistory} from './repo-issue-content.js';
import {initDiffFileTree} from './repo-diff-filetree.js';
import {initDiffCommitSelect} from './repo-diff-commitselect.js';
import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.js';
import {initViewedCheckboxListenerFor, countAndUpdateViewedFiles, initExpandAndCollapseFilesButton} from './pull-view-file.js';
import {initImageDiff} from './imagediff.js';

const {csrfToken, pageData} = window.config;

function initRepoDiffReviewButton() {
  const $reviewBox = $('#review-box');
  const $counter = $reviewBox.find('.review-comments-counter');

  $(document).on('click', 'button[name="pending_review"]', (e) => {
    const $form = $(e.target).closest('form');
    // Watch for the form's submit event.
    $form.on('submit', () => {
      const num = parseInt($counter.attr('data-pending-comment-number')) + 1 || 1;
      $counter.attr('data-pending-comment-number', num);
      $counter.text(num);
      // Force the browser to reflow the DOM. This is to ensure that the browser replay the animation
      $reviewBox.removeClass('pulse');
      $reviewBox.width();
      $reviewBox.addClass('pulse');
    });
  });
}

function initRepoDiffFileViewToggle() {
  $('.file-view-toggle').on('click', function () {
    const $this = $(this);
    $this.parent().children().removeClass('active');
    $this.addClass('active');

    const $target = $($this.data('toggle-selector'));
    $target.parent().children().addClass('gt-hidden');
    $target.removeClass('gt-hidden');
  });
}

function initRepoDiffConversationForm() {
  $(document).on('submit', '.conversation-holder form', async (e) => {
    e.preventDefault();

    const $form = $(e.target);
    const $textArea = $form.find('textarea');
    if (!validateTextareaNonEmpty($textArea)) {
      return;
    }

    const formData = new FormData($form[0]);

    // if the form is submitted by a button, append the button's name and value to the form data
    const submitter = e.originalEvent?.submitter;
    const isSubmittedByButton = (submitter?.nodeName === 'BUTTON') || (submitter?.nodeName === 'INPUT' && submitter.type === 'submit');
    if (isSubmittedByButton && submitter.name) {
      formData.append(submitter.name, submitter.value);
    }
    const formDataString = String(new URLSearchParams(formData));
    const $newConversationHolder = $(await $.post($form.attr('action'), formDataString));
    const {path, side, idx} = $newConversationHolder.data();

    $form.closest('.conversation-holder').replaceWith($newConversationHolder);
    if ($form.closest('tr').data('line-type') === 'same') {
      $(`[data-path="${path}"] .add-code-comment[data-idx="${idx}"]`).addClass('gt-invisible');
    } else {
      $(`[data-path="${path}"] .add-code-comment[data-side="${side}"][data-idx="${idx}"]`).addClass('gt-invisible');
    }
    $newConversationHolder.find('.dropdown').dropdown();
    initCompReactionSelector($newConversationHolder);
  });

  $(document).on('click', '.resolve-conversation', async function (e) {
    e.preventDefault();
    const comment_id = $(this).data('comment-id');
    const origin = $(this).data('origin');
    const action = $(this).data('action');
    const url = $(this).data('update-url');

    const data = await $.post(url, {_csrf: csrfToken, origin, action, comment_id});

    if ($(this).closest('.conversation-holder').length) {
      const conversation = $(data);
      $(this).closest('.conversation-holder').replaceWith(conversation);
      conversation.find('.dropdown').dropdown();
      initCompReactionSelector(conversation);
    } else {
      window.location.reload();
    }
  });
}

export function initRepoDiffConversationNav() {
  // Previous/Next code review conversation
  $(document).on('click', '.previous-conversation', (e) => {
    const $conversation = $(e.currentTarget).closest('.comment-code-cloud');
    const $conversations = $('.comment-code-cloud:not(.gt-hidden)');
    const index = $conversations.index($conversation);
    const previousIndex = index > 0 ? index - 1 : $conversations.length - 1;
    const $previousConversation = $conversations.eq(previousIndex);
    const anchor = $previousConversation.find('.comment').first().attr('id');
    window.location.href = `#${anchor}`;
  });
  $(document).on('click', '.next-conversation', (e) => {
    const $conversation = $(e.currentTarget).closest('.comment-code-cloud');
    const $conversations = $('.comment-code-cloud:not(.gt-hidden)');
    const index = $conversations.index($conversation);
    const nextIndex = index < $conversations.length - 1 ? index + 1 : 0;
    const $nextConversation = $conversations.eq(nextIndex);
    const anchor = $nextConversation.find('.comment').first().attr('id');
    window.location.href = `#${anchor}`;
  });
}

// Will be called when the show more (files) button has been pressed
function onShowMoreFiles() {
  initRepoIssueContentHistory();
  initViewedCheckboxListenerFor();
  countAndUpdateViewedFiles();
  initImageDiff();
}

export function loadMoreFiles(url) {
  const $target = $('a#diff-show-more-files');
  if ($target.hasClass('disabled') || pageData.diffFileInfo.isLoadingNewData) {
    return;
  }

  pageData.diffFileInfo.isLoadingNewData = true;
  $target.addClass('disabled');
  $.ajax({
    type: 'GET',
    url,
  }).done((resp) => {
    const $resp = $(resp);
    // the response is a full HTML page, we need to extract the relevant contents:
    // 1. append the newly loaded file list items to the existing list
    $('#diff-incomplete').replaceWith($resp.find('#diff-file-boxes').children());
    // 2. re-execute the script to append the newly loaded items to the JS variables to refresh the DiffFileTree
    $('body').append($resp.find('script#diff-data-script'));

    onShowMoreFiles();
  }).always(() => {
    $target.removeClass('disabled');
    pageData.diffFileInfo.isLoadingNewData = false;
  });
}

function initRepoDiffShowMore() {
  $(document).on('click', 'a#diff-show-more-files', (e) => {
    e.preventDefault();

    const $target = $(e.target);
    const linkLoadMore = $target.attr('data-href');
    loadMoreFiles(linkLoadMore);
  });

  $(document).on('click', 'a.diff-load-button', (e) => {
    e.preventDefault();
    const $target = $(e.target);

    if ($target.hasClass('disabled')) {
      return;
    }

    $target.addClass('disabled');

    const url = $target.data('href');
    $.ajax({
      type: 'GET',
      url,
    }).done((resp) => {
      if (!resp) {
        $target.removeClass('disabled');
        return;
      }
      $target.parent().replaceWith($(resp).find('#diff-file-boxes .diff-file-body .file-body').children());
      onShowMoreFiles();
    }).fail(() => {
      $target.removeClass('disabled');
    });
  });
}

export function initRepoDiffView() {
  initRepoDiffConversationForm();
  const diffFileList = $('#diff-file-list');
  if (diffFileList.length === 0) return;
  initDiffFileTree();
  initDiffCommitSelect();
  initRepoDiffShowMore();
  initRepoDiffReviewButton();
  initRepoDiffFileViewToggle();
  initViewedCheckboxListenerFor();
  initExpandAndCollapseFilesButton();
}
