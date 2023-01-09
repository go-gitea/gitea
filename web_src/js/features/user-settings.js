import $ from 'jquery';
import {showTemporaryTooltip} from '../modules/tippy.js';

const {appSubUrl, csrfToken} = window.config;

export function initUserSettings() {
  if ($('.user.settings.profile').length > 0) {
    $('#username').on('keyup', function () {
      const $prompt = $('#name-change-prompt');
      const $prompt_redirect = $('#name-change-redirect-prompt');
      if ($(this).val().toString().toLowerCase() !== $(this).data('name').toString().toLowerCase()) {
        $prompt.show();
        $prompt_redirect.show();
      } else {
        $prompt.hide();
        $prompt_redirect.hide();
      }
    });

    $('.selection').dropdown({
      onChange: (_text, _value, $choice) => {
        const $this = $choice.parent().parent();
        const input = $this.find('input');
        console.info($this, input);
        $.ajax({
          url: `${appSubUrl}/user/settings/config`,
          type: 'POST',
          data: {
            _csrf: csrfToken,
            key: input.attr('name'),
            value: input.attr('value'),
          }
        }).done((resp) => {
          if (resp) {
            if (resp.redirect) {
              window.location.href = resp.redirect;
            } else if (resp.version) {
              input.attr('version', resp.version);
            } else if (resp.err) {
              showTemporaryTooltip($this, resp.err);
            }
          }
        });
        return false;
      }
    });
  }
}
