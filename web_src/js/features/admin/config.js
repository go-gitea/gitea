import $ from 'jquery';
import {showTemporaryTooltip} from '../../modules/tippy.js';

const {appSubUrl, csrfToken} = window.config;

export function initAdminConfigs() {
  const $adminConfig = $('.page-content.admin.config');
  if (!$adminConfig.length) return;

  $("input[type='checkbox']").on('change', (e) => {
    const $this = $(e.currentTarget);
    $.ajax({
      url: `${appSubUrl}/admin/config`,
      type: 'POST',
      data: {
        _csrf: csrfToken,
        key: $this.attr('name'),
        value: $this.is(':checked'),
      }
    }).done((resp) => {
      if (resp) {
        if (resp.redirect) {
          window.location.href = resp.redirect;
        } else if (resp.err) {
          showTemporaryTooltip(e.currentTarget, resp.err);
          $this.prop('checked', !$this.is(':checked'));
        }
      }
    });
  });
}
