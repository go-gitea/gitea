import $ from 'jquery';
import {initCompLabelEdit} from './comp/LabelEdit.js';
import {hideElem, showElem} from '../utils/dom.js';

export function initCommonOrganization() {
  if ($('.organization').length === 0) {
    return;
  }

  if ($('.organization.settings.options').length > 0) {
    $('#org_name').on('keyup', function () {
      const $prompt = $('#org-name-change-prompt');
      const $prompt_redirect = $('#org-name-change-redirect-prompt');
      if ($(this).val().toString().toLowerCase() !== $(this).data('org-name').toString().toLowerCase()) {
        showElem($prompt);
        showElem($prompt_redirect);
      } else {
        hideElem($prompt);
        hideElem($prompt_redirect);
      }
    });
  }

  // Labels
  initCompLabelEdit('.organization.settings.labels');
}
