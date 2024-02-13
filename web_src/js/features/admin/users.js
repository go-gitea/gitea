export function initAdminUserListSearchForm() {
  const searchForm = window.config.pageData.adminUserListSearchForm;
  if (!searchForm) return;

  const form = document.querySelector('#user-list-search-form');
  if (!form) return;

  for (const button of form.querySelectorAll(`button[name=sort][value="${searchForm.SortType}"]`)) {
    button.classList.add('active');
  }

  if (searchForm.StatusFilterMap) {
    for (const [k, v] of Object.entries(searchForm.StatusFilterMap)) {
      if (!v) continue;
      for (const input of form.querySelectorAll(`input[name="status_filter[${k}]"][value="${v}"]`)) {
        input.checked = true;
      }
    }
  }

  for (const radio of form.querySelectorAll('input[type=radio]')) {
    radio.addEventListener('click', () => {
      form.submit();
    });
  }

  const resetButtons = form.querySelectorAll('.j-reset-status-filter');
  for (const button of resetButtons) {
    button.addEventListener('click', (e) => {
      e.preventDefault();
      for (const input of form.querySelectorAll('input[type=radio]')) {
        if (input.name.startsWith('status_filter[')) {
          input.checked = false;
        }
      }
      form.submit();
    });
  }
}
