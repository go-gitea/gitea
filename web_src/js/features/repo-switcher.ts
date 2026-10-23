import {initDropdown} from '../modules/dropdown.ts';
import {GET} from '../modules/fetch.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {svg} from '../svg.ts';

type RepoItem = {full_name: string; html_url: string; private: boolean; fork: boolean};
type RepoSearchResponse = {data: Array<{repository: RepoItem}>};

function repoIcon(repo: RepoItem): string {
  if (repo.private) return svg('octicon-lock', 16);
  if (repo.fork) return svg('octicon-repo-forked', 16);
  return svg('octicon-repo', 16);
}

export function initRepoSwitcher(el: HTMLElement) {
  const currentFullName = el.getAttribute('data-current-full-name');
  initDropdown(el, {
    async loadItems(query, signal) {
      const response = await GET(el.getAttribute('data-query-url')!.replace('{query}', () => encodeURIComponent(query)), {signal});
      const {data} = await response.json() as RepoSearchResponse;
      return data.map(({repository: repo}) => createElementFromHTML(html`
        <a class="dropdown-item ${repo.full_name === currentFullName ? 'active' : ''}" href="${repo.html_url}">
          <span class="tw-w-[16px]">${htmlRaw(repo.full_name === currentFullName ? svg('octicon-check', 16) : '')}</span>
          <span>${htmlRaw(repoIcon(repo))}</span>
          <span class="gt-ellipsis">${repo.full_name.split('/')[1]}</span>
        </a>
      `));
    },
  });
}
