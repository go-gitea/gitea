export function initRepositorySearch() {
  const repositorySearchForm = document.querySelector('#repo-search-form');
  if (!repositorySearchForm) return;

  repositorySearchForm.addEventListener('change', (e) => {
    e.preventDefault();

    const formData = new FormData(repositorySearchForm);
    const params = new URLSearchParams(formData);

    if (e.target.name === 'clear-filter') {
      params.delete('archived');
      params.delete('fork');
      params.delete('mirror');
      params.delete('template');
      params.delete('private');
    }

    params.delete('clear-filter');
    window.location.search = params.toString();
  });
}
