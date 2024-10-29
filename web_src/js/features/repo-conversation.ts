import {GET, POST} from '../modules/fetch.ts';

export function initRepoConversationCommentDelete() {
    // Delete comment
    document.addEventListener('click', async (e) => {
      if (!e.target.matches('.delete-comment')) return;
      if (!e.target.matches('.conversation-comment')) return;
      e.preventDefault();
  
      const deleteButton = e.target;
      if (window.confirm(deleteButton.getAttribute('data-locale'))) {
        try {
          const response = await POST(deleteButton.getAttribute('data-url'));
          if (!response.ok) throw new Error('Failed to delete comment');
  
          const conversationHolder = deleteButton.closest('.conversation-holder');
          const parentTimelineItem = deleteButton.closest('.timeline-item');
          const parentTimelineGroup = deleteButton.closest('.timeline-item-group');
  
          // Check if this was a pending comment.
          if (conversationHolder?.querySelector('.pending-label')) {
            const counter = document.querySelector('#review-box .review-comments-counter');
            let num = parseInt(counter?.getAttribute('data-pending-comment-number')) - 1 || 0;
            num = Math.max(num, 0);
            counter.setAttribute('data-pending-comment-number', num);
            counter.textContent = String(num);
          }
  
          document.querySelector(`#${deleteButton.getAttribute('data-comment-id')}`)?.remove();
  
          if (conversationHolder && !conversationHolder.querySelector('.comment')) {
            const path = conversationHolder.getAttribute('data-path');
            const side = conversationHolder.getAttribute('data-side');
            const idx = conversationHolder.getAttribute('data-idx');
            const lineType = conversationHolder.closest('tr')?.getAttribute('data-line-type');
  
            // the conversation holder could appear either on the "Conversation" page, or the "Files Changed" page
            // on the Conversation page, there is no parent "tr", so no need to do anything for "add-code-comment"
            if (lineType) {
              if (lineType === 'same') {
                document.querySelector(`[data-path="${path}"] .add-code-comment[data-idx="${idx}"]`).classList.remove('tw-invisible');
              } else {
                document.querySelector(`[data-path="${path}"] .add-code-comment[data-side="${side}"][data-idx="${idx}"]`).classList.remove('tw-invisible');
              }
            }
            conversationHolder.remove();
          }
  
          // Check if there is no review content, move the time avatar upward to avoid overlapping the content below.
          if (!parentTimelineGroup?.querySelector('.timeline-item.comment') && !parentTimelineItem?.querySelector('.conversation-holder')) {
            const timelineAvatar = parentTimelineGroup?.querySelector('.timeline-avatar');
            timelineAvatar?.classList.remove('timeline-avatar-offset');
          }
        } catch (error) {
          console.error(error);
        }
      }
    });
  }