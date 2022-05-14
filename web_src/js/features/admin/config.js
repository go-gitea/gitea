import $ from 'jquery';

const {appSubUrl, csrfToken} = window.config;

export function initAdminConfigs() {
  const isAdminConfigPage = window.config.pageData.adminConfigPage;
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
        console.info(resp);
        if (resp.redirect) {
          window.location.href = resp.redirect;
        } else if (resp.version) {
          $this.attr('version', resp.version);
        }
      }
    });

    e.preventDefault();
    return false;
  });
}
