import $ from 'jquery';
import {showTemporaryTooltip} from '../../modules/tippy.js';

const {appSubUrl, csrfToken, pageData} = window.config;

export function initAdminConfigs() {
  const isAdminConfigPage = pageData?.adminConfigPage;
  if (!isAdminConfigPage) return;

  $("input[type='checkbox']").on('change', (e) => {
    const $this = $(e.currentTarget);
    $.ajax({
      url: `${appSubUrl}/admin/config`,
      type: 'POST',
      data: {
        _csrf: csrfToken,
        key: $this.attr('name'),
        value: $this.is(':checked'),
        version: $this.attr('version'),
      }
    }).done((resp) => {
      if (resp) {
        if (resp.redirect) {
          window.location.href = resp.redirect;
        } else if (resp.version) {
          $this.attr('version', resp.version);
        } else if (resp.err) {
          showTemporaryTooltip(e.currentTarget, resp.err);
          $this.prop('checked', !$this.is(':checked'));
        }
      }
    });

    e.preventDefault();
    return false;
  });
}
