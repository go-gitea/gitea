import $ from 'jquery';
import {POST} from '../../modules/fetch.js';

export function initCompReactionSelector($parent) {
  $parent.find(`.select-reaction .item.reaction, .comment-reaction-button`).on('click', async function (e) {
    e.preventDefault();

    if (this.classList.contains('disabled')) return;

    const actionUrl = this.closest('[data-action-url]')?.getAttribute('data-action-url');
    const reactionContent = this.getAttribute('data-reaction-content');
    const hasReacted = this.closest('.ui.segment.reactions')?.querySelector(`a[data-reaction-content="${reactionContent}"]`)?.getAttribute('data-has-reacted') === 'true';

    const res = await POST(`${actionUrl}/${hasReacted ? 'unreact' : 'react'}`, {
      data: new URLSearchParams({content: reactionContent}),
    });

    const data = await res.json();
    if (data && (data.html || data.empty)) {
      const $content = $(this).closest('.content');
      let $react = $content.find('.segment.reactions');
      if ((!data.empty || data.html === '') && $react.length > 0) {
        $react.remove();
      }
      if (!data.empty) {
        const $attachments = $content.find('.segment.bottom:first');
        $react = $(data.html);
        if ($attachments.length > 0) {
          $react.insertBefore($attachments);
        } else {
          $react.appendTo($content);
        }
        $react.find('.dropdown').dropdown();
        initCompReactionSelector($react);
      }
    }
  });
}
