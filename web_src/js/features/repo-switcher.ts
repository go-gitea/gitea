import {fomanticQuery} from '../modules/fomantic/base.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {svg} from '../svg.ts';

type RepoItem = {full_name: string; html_url: string; private: boolean; fork: boolean};
type RepoSearchResponse = {data: Array<{repository: RepoItem}>};

function repoIcon(repo: RepoItem): string {
  if (repo.private) return svg('octicon-lock', 16);
  if (repo.fork) return svg('octicon-repo-forked', 16);
  return svg('octicon-repo', 16);
}

// quick repo switcher: the caret next to the repo name opens a remote-search dropdown
// listing the owner's repositories, and navigates to the selected one
export function initRepoSwitcher(el: HTMLElement) {
  const currentFullName = el.getAttribute('data-current-full-name');
  const $dropdown = fomanticQuery(el);
  $dropdown.dropdown({
    showOnFocus: false, // don't auto popup the dropdown menu, avoid interrupting users keyboard navigation on the page
    apiSettings: {
      url: el.getAttribute('data-query-url')!,
      onResponse: (response: RepoSearchResponse) => ({results: response.data.map(({repository: repo}) => {
        const svgCheck = repo.full_name === currentFullName ? svg('octicon-check', 16) : '';
        const svgRepoIcon = repoIcon(repo);
        return {
          type: 'html',
          html: html`
            <a class="item" href="${repo.html_url}">
              <span class="tw-w-[16px]">${htmlRaw(svgCheck)}</span>
              <span>${htmlRaw(svgRepoIcon)}</span>
              <span class="gt-ellipsis">${repo.full_name.split('/')[1]}</span>
            </a>
          `,
        };
      })}),
    },
  });
}
