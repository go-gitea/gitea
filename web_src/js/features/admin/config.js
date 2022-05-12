import $ from 'jquery';

const {appSubUrl, csrfToken} = window.config;

export function initAdminConfigs() {
  const isAdminConfigPage = window.config.pageData.adminConfigPage;
  if (!isAdminConfigPage) return;

  $("input[type='checkbox']").on('change', (e) => {
    const $this = $(e.currentTarget);
    $.ajax({
      type: 'POST',
      url: `${appSubUrl}/admin/config`,
      data: {
        _csrf: csrfToken,
        key: $this.attr("name"),
        value: $this.is(':checked'),
      },
      success: (data, _, jqXHR) => {

      }
    });

    e.preventDefault();
    return false;
  });
}
