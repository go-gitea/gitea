import {initCompLabelEdit} from './comp/LabelEdit.js';

export function initCommonOrganization() {
  if ($('.organization').length === 0) {
    return;
  }

  if ($('.organization.settings.options').length > 0) {
    $('#org_name').on('keyup', function () {
      const $prompt = $('#org-name-change-prompt');
      const $prompt_redirect = $('#org-name-change-redirect-prompt');
      if ($(this).val().toString().toLowerCase() !== $(this).data('org-name').toString().toLowerCase()) {
        $prompt.show();
        $prompt_redirect.show();
      } else {
        $prompt.hide();
        $prompt_redirect.hide();
      }
    });
  }

  // Labels
  initCompLabelEdit('.organization.settings.labels');
}
