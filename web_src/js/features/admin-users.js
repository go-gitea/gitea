export function initAdminUserListSearchForm() {
  const searchForm = window.config.PageData.adminUserListSearchForm;
  if (!searchForm) return;

  const $form = $('#user-list-search-form');
  if (!$form.length) return;

  $form.find(`button[name=sort][value=${searchForm.SortType}]`).addClass('active');

  if (searchForm.StatusFilterMap) {
    for (const [k, v] of Object.entries(searchForm.StatusFilterMap)) {
      if (!v) continue;
      $form.find(`input[name="status_filter[${k}]"][value=${v}]`).prop('checked', true);
    }
  }

  $form.find(`input[type=radio]`).click(() => {
    $form.submit();
    return false;
  });

  $form.find('.j-reset-status-filter').click(() => {
    $form.find(`input[type=radio]`).each((_, e) => {
      const $e = $(e);
      if ($e.attr('name').startsWith('status_filter[')) {
        $e.prop('checked', false);
      }
    });
    $form.submit();
    return false;
  });
}
