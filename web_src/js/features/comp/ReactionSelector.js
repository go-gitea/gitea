import $ from 'jquery';
import {POST} from '../../modules/fetch.js';

export function initCompReactionSelector() {
  const containers = document.querySelectorAll('.comment-list, .diff-file-body');
  if (!containers.length) return;

  for (const container of containers) {
    container.addEventListener('click', async (e) => {
      const item = e.target.matches('.item.reaction') ? e.target : e.target.closest('.item.reaction');
      const button = e.target.matches('.comment-reaction-button') ? e.target : e.target.closest('.comment-reaction-button');
      if (!item && !button) return;
      e.preventDefault();
      const target = item || button;
      if (target.classList.contains('disabled')) return;

      const actionUrl = target.closest('[data-action-url]')?.getAttribute('data-action-url');
      const reactionContent = target.getAttribute('data-reaction-content');
      const hasReacted = target.closest('.segment.reactions')?.querySelector(`a[data-reaction-content="${CSS.escape(reactionContent)}"]`)?.getAttribute('data-has-reacted') === 'true';
      const content = target.closest('.content');

      const res = await POST(`${actionUrl}/${hasReacted ? 'unreact' : 'react'}`, {
        data: new URLSearchParams({content: reactionContent}),
      });

      const data = await res.json();
      if (data && (data.html || data.empty)) {
        const reactions = content.querySelector('.segment.reactions');
        if ((!data.empty || data.html === '') && reactions) {
          reactions.remove();
        }
        if (!data.empty) {
          const attachments = content.querySelector('.segment.bottom');
          if (attachments) {
            attachments.insertAdjacentHTML('beforebegin', data.html);
          } else {
            content.insertAdjacentHTML('beforeend', data.html);
          }
          $(content.querySelectorAll('.segment.reactions .dropdown')).dropdown();
        }
      }
    });
  }
}
