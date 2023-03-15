import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';

export function initUserSettings() {
  if ($('.user.settings.profile').length > 0) {
    $('#username').on('keyup', function () {
      const $prompt = $('#name-change-prompt');
      const $prompt_redirect = $('#name-change-redirect-prompt');
      if ($(this).val().toString().toLowerCase() !== $(this).data('name').toString().toLowerCase()) {
        showElem($prompt);
        showElem($prompt_redirect);
      } else {
        hideElem($prompt);
        hideElem($prompt_redirect);
      }
    });
  }
}
