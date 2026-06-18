import {attachSearchBox} from '../modules/search.ts';
import {svg} from '../svg.ts';

const {appSubUrl} = window.config;

type RepoItem = {full_name: string; html_url: string; private: boolean; fork: boolean};
type RepoSearchResponse = {data: Array<{repository: RepoItem}>};

function repoIcon(repo: RepoItem): string {
  if (repo.private) return svg('octicon-lock', 16);
  if (repo.fork) return svg('octicon-repo-forked', 16);
  return svg('octicon-repo', 16);
}

// GitHub-style quick repo switcher: a caret next to the repo name opens a popover
// that lists/searches the current owner's repositories and navigates to the selection.
export function initRepoSwitcher(el: HTMLElement) {
  const button = el.querySelector<HTMLButtonElement>('.repo-switcher-button')!;
  const menu = el.querySelector<HTMLElement>('.repo-switcher-menu')!;
  const search = el.querySelector<HTMLElement>('.ui.search')!;
  const input = search.querySelector<HTMLInputElement>('input.prompt')!;
  const uid = el.getAttribute('data-uid');
  const currentFullName = el.getAttribute('data-current-full-name');

  // owner-scoped search; minCharacters:0 so opening shows the owner's repos before typing
  const url = `${appSubUrl}/repo/search?q={query}&uid=${uid}&priority_owner_id=${uid}`;
  const box = attachSearchBox(search, url, (response: RepoSearchResponse) => response.data.map((item) => ({
    title: item.repository.full_name.split('/')[1],
    description: item.repository.full_name,
    link: item.repository.html_url,
    icon: repoIcon(item.repository),
    active: item.repository.full_name === currentFullName,
  })), {
    minCharacters: 0,
    onSelect: (result) => {
      if (result.link) window.location.href = result.link;
    },
  });

  let open = false;
  const closeMenu = () => {
    menu.classList.add('tw-hidden');
    open = false;
  };
  const openMenu = () => {
    menu.classList.remove('tw-hidden');
    open = true;
    input.focus();
    box.refresh(); // immediately load the owner's repos (empty query)
  };

  button.addEventListener('click', (e) => {
    e.stopPropagation();
    if (open) {
      closeMenu();
    } else {
      openMenu();
    }
  });
  document.addEventListener('click', (e) => {
    if (open && !el.contains(e.target as Node)) closeMenu();
  });
  el.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closeMenu();
  });
}
