import type {DOMEvent} from '../utils/dom.ts';

export function initRepositorySearch() {
  const repositorySearchForm = document.querySelector<HTMLFormElement>('#repo-search-form');
  if (!repositorySearchForm) return;

  repositorySearchForm.addEventListener('change', (e: DOMEvent<Event, HTMLInputElement>) => {
    e.preventDefault();

    const params = new URLSearchParams();
    for (const [key, value] of new FormData(repositorySearchForm).entries()) {
      params.set(key, value.toString());
    }
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
