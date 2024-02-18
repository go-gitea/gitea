export function initRepositorySearch() {
  const repositorySearchForm = document.querySelector('#repo-search-form');
  if (!repositorySearchForm) return;

  for (const radio of repositorySearchForm.querySelectorAll('input[type=radio]')) {
    radio.addEventListener('click', (e) => {
      e.preventDefault();

      const formData = new FormData(repositorySearchForm);
      const params = new URLSearchParams(formData);
      const otherQueryParams = repositorySearchForm.getAttribute('data-query-params');
      window.location.search = `${otherQueryParams}&${params.toString()}`;
    });
  }
}
