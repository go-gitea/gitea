export function initAdminUserListSearchForm(): void {
  const searchForm = window.config.pageData.adminUserListSearchForm;
  if (!searchForm) return;

  const form = document.querySelector<HTMLFormElement>('#user-list-search-form');
  if (!form) return;

  for (const button of form.querySelectorAll(`button[name=sort][value="${CSS.escape(searchForm.SortType)}"]`)) {
    button.classList.add('active');
  }

  for (const [k, v] of Object.entries(searchForm.StatusFilterMap || {})) {
    if (!v) continue;
    for (const input of form.querySelectorAll<HTMLInputElement>(`input[name="status_filter[${CSS.escape(k)}]"][value="${CSS.escape(v)}"]`)) {
      input.checked = true;
    }
  }

  if (searchForm.UserTypeFilter) {
    for (const input of form.querySelectorAll<HTMLInputElement>(`input[name="user_type"][value="${CSS.escape(searchForm.UserTypeFilter)}"]`)) {
      input.checked = true;
    }
  }

  for (const radio of form.querySelectorAll<HTMLInputElement>('input[type=radio]')) {
    radio.addEventListener('click', () => {
      form.submit();
    });
  }

  const resetFilter = (selector: string, matchName: (name: string) => boolean) => {
    for (const button of form.querySelectorAll<HTMLAnchorElement>(selector)) {
      button.addEventListener('click', (e) => {
        e.preventDefault();
        for (const input of form.querySelectorAll<HTMLInputElement>('input[type=radio]')) {
          if (matchName(input.name)) {
            input.checked = false;
          }
        }
        form.submit();
      });
    }
  };

  resetFilter('.j-reset-status-filter', (name) => name.startsWith('status_filter['));
  resetFilter('.j-reset-type-filter', (name) => name === 'user_type');
}
