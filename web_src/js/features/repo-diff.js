import {initCompReactionSelector} from './comp/ReactionSelector.js';

const {csrfToken} = window.config;

export function initRepoDiffReviewButton() {
  $(document).on('click', 'button[name="is_review"]', (e) => {
    $(e.target).closest('form').append('<input type="hidden" name="is_review" value="true">');
  });
}

export function initRepoDiffFileViewToggle() {
  $('.file-view-toggle').on('click', function () {
    const $this = $(this);
    $this.parent().children().removeClass('active');
    $this.addClass('active');

    const $target = $($this.data('toggle-selector'));
    $target.parent().children().addClass('hide');
    $target.removeClass('hide');
  });
}

export function initRepoDiffConversationForm() {
  $(document).on('submit', '.conversation-holder form', async (e) => {
    e.preventDefault();
    const form = $(e.target);
    const newConversationHolder = $(await $.post(form.attr('action'), form.serialize()));
    const {path, side, idx} = newConversationHolder.data();

    form.closest('.conversation-holder').replaceWith(newConversationHolder);
    if (form.closest('tr').data('line-type') === 'same') {
      $(`a.add-code-comment[data-path="${path}"][data-idx="${idx}"]`).addClass('invisible');
    } else {
      $(`a.add-code-comment[data-path="${path}"][data-side="${side}"][data-idx="${idx}"]`).addClass('invisible');
    }
    newConversationHolder.find('.dropdown').dropdown();
    initCompReactionSelector(newConversationHolder);
  });


  $('.resolve-conversation').on('click', async function (e) {
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
    const $conversations = $('.comment-code-cloud:not(.hide)');
    const index = $conversations.index($conversation);
    const previousIndex = index > 0 ? index - 1 : $conversations.length - 1;
    const $previousConversation = $conversations.eq(previousIndex);
    const anchor = $previousConversation.find('.comment').first().attr('id');
    window.location.href = `#${anchor}`;
  });
  $(document).on('click', '.next-conversation', (e) => {
    const $conversation = $(e.currentTarget).closest('.comment-code-cloud');
    const $conversations = $('.comment-code-cloud:not(.hide)');
    const index = $conversations.index($conversation);
    const nextIndex = index < $conversations.length - 1 ? index + 1 : 0;
    const $nextConversation = $conversations.eq(nextIndex);
    const anchor = $nextConversation.find('.comment').first().attr('id');
    window.location.href = `#${anchor}`;
  });
}
