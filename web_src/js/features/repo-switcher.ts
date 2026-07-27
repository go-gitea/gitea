import {fomanticQuery} from '../modules/fomantic/base.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {svg} from '../svg.ts';

const {appSubUrl} = window.config;

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
  const uid = el.getAttribute('data-uid');
  const currentFullName = el.getAttribute('data-current-full-name');
  const $dropdown = fomanticQuery(el);
  $dropdown.dropdown({
    action: 'hide', // no selection is kept, choosing a repo navigates away
    forceSelection: false, // otherwise closing with Escape commits the highlighted item and navigates
    minCharacters: 0, // an empty query lists the owner's repositories when the menu opens
    showOnFocus: false, // reopen from the loaded list instead of blocking on another query
    saveRemoteData: false,
    apiSettings: {
      cache: false,
      throttleFirstRequest: false, // open without waiting out the throttle, later keystrokes stay debounced
      url: `${appSubUrl}/repo/search?q={query}&uid=${uid}&priority_owner_id=${uid}`,
      onResponse: (response: RepoSearchResponse) => ({results: response.data.map(({repository: repo}) => {
        const check = repo.full_name === currentFullName ? svg('octicon-check', 16) : '';
        return {
          value: repo.html_url,
          name: html`<span class="repo-switcher-check">${htmlRaw(check)}</span><span class="repo-switcher-icon">${htmlRaw(repoIcon(repo))}</span><span class="gt-ellipsis">${repo.full_name.split('/')[1]}</span>`,
        };
      })}),
    },
    onChange(value: string) {
      if (value) window.location.assign(value);
    },
    onHide() {
      // drop the last query's results so the next open shows the full list right away
      setTimeout(() => $dropdown.dropdown('search', ''), 0);
    },
  });
}
