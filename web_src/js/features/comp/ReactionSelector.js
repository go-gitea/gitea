import $ from 'jquery';
import {createTippy} from '../../modules/tippy.js';

const {csrfToken} = window.config;

export function initCompReactionSelector(parent) {
  let selector = 'a.label';
  if (!parent) {
    parent = $(document);
    selector = `.reactions ${selector}`;
  }

  for (const el of parent[0].querySelectorAll(selector)) {
    createTippy(el, {
      placement: 'bottom-start',
      content: el.getAttribute('data-title'),
      theme: 'tooltip',
      hideOnClick: true,
    });
  }

  parent.find(`.select-reaction > .menu .item, ${selector}`).on('click', async function (e) {
    e.preventDefault();

    if ($(this).hasClass('disabled')) return;

    const reactionContent = $(this).attr('data-reaction-content');
    let actionUrl, hasReacted;
    if ($(this).hasClass('item')) { // in dropdown menu
      actionUrl = $(this).closest('.select-reaction').data('action-url');
      const parent = $(this).closest('.segment.reactions');
      const el = parent.find(`[data-reaction-content="${reactionContent}"]`);
      hasReacted = el.attr('data-has-reacted') === 'true';
    } else { // outside of dropdown menu
      actionUrl = $(this).data('action-url');
      hasReacted = $(this).attr('data-has-reacted') === 'true';
    }

    const res = await fetch(`${actionUrl}/${hasReacted ? 'unreact' : 'react'}`, {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
      },
      body: new URLSearchParams({
        _csrf: csrfToken,
        content: reactionContent,
      }),
    });

    const data = await res.json();
    if (data && (data.html || data.empty)) {
      const content = $(this).closest('.content');
      let react = content.find('.segment.reactions');
      if ((!data.empty || data.html === '') && react.length > 0) {
        react.remove();
      }
      if (!data.empty) {
        react = $('<div class="ui attached segment reactions"></div>');
        const attachments = content.find('.segment.bottom:first');
        if (attachments.length > 0) {
          react.insertBefore(attachments);
        } else {
          react.appendTo(content);
        }
        react.html(data.html);
        react.find('.dropdown').dropdown();
        initCompReactionSelector(react);
      }
    }
  });
}
