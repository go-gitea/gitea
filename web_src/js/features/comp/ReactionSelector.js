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
    createTippy(el, {placement: 'bottom-start', content: el.getAttribute('data-title')});
  }

  parent.find(`.select-reaction > .menu > .item, ${selector}`).on('click', function (e) {
    e.preventDefault();

    if ($(this).hasClass('disabled')) return;

    const actionURL = $(this).hasClass('item') ? $(this).closest('.select-reaction').data('action-url') : $(this).data('action-url');
    const url = `${actionURL}/${$(this).hasClass('primary') ? 'unreact' : 'react'}`;
    $.ajax({
      type: 'POST',
      url,
      data: {
        _csrf: csrfToken,
        content: $(this).attr('data-reaction-content'),
      }
    }).done((resp) => {
      if (resp && (resp.html || resp.empty)) {
        const content = $(this).closest('.content');
        let react = content.find('.segment.reactions');
        if ((!resp.empty || resp.html === '') && react.length > 0) {
          react.remove();
        }
        if (!resp.empty) {
          react = $('<div class="ui attached segment reactions"></div>');
          const attachments = content.find('.segment.bottom:first');
          if (attachments.length > 0) {
            react.insertBefore(attachments);
          } else {
            react.appendTo(content);
          }
          react.html(resp.html);
          react.find('.dropdown').dropdown();
          initCompReactionSelector(react);
        }
      }
    });
  });
}
