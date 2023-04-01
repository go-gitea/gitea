import $ from 'jquery';
import {hideElem, showElem, toggleElem} from '../../utils/dom.js';

const {csrfToken} = window.config;

export function initCompWebHookEditor() {
  if ($('.new.webhook').length === 0) {
    return;
  }

  $('.events.checkbox input').on('change', function () {
    if ($(this).is(':checked')) {
      showElem($('.events.fields'));
    }
  });
  $('.non-events.checkbox input').on('change', function () {
    if ($(this).is(':checked')) {
      hideElem($('.events.fields'));
    }
  });

  const updateContentType = function () {
    const visible = $('#http_method').val() === 'POST';
    toggleElem($('#content_type').parent().parent(), visible);
  };
  updateContentType();
  $('#http_method').on('change', () => {
    updateContentType();
  });

  // Test delivery
  $('#test-delivery').on('click', function () {
    const $this = $(this);
    $this.addClass('loading disabled');
    $.post($this.data('link'), {
      _csrf: csrfToken
    }).done(
      setTimeout(() => {
        window.location.href = $this.data('redirect');
      }, 5000)
    );
  });
}
