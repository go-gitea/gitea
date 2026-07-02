import {GET} from '../modules/fetch.ts';
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

export function initRepoSwitcher(el: HTMLElement) {
  const button = el.querySelector<HTMLButtonElement>('.repo-switcher-button')!;
  const menu = el.querySelector<HTMLElement>('.repo-switcher-menu')!;
  const list = el.querySelector<HTMLElement>('.repo-switcher-list')!;
  const uid = el.getAttribute('data-uid');
  const currentFullName = el.getAttribute('data-current-full-name');

  const url = `${appSubUrl}/repo/search?q=&uid=${uid}&priority_owner_id=${uid}`;
  let loaded = false;

  const load = async () => {
    if (loaded) return;
    const response = await GET(url);
    if (!response.ok) return;
    const {data}: RepoSearchResponse = await response.json();
    list.replaceChildren(...data.map(({repository: repo}) => {
      const item = document.createElement('a');
      item.className = 'repo-switcher-item';
      if (repo.full_name === currentFullName) item.classList.add('active');
      item.href = repo.html_url;
      item.innerHTML = html`<span class="result-check">${htmlRaw(repo.full_name === currentFullName ? svg('octicon-check', 16) : '')}</span><span class="result-icon">${htmlRaw(repoIcon(repo))}</span><span>${repo.full_name.split('/')[1]}</span>`;
      return item;
    }));
    loaded = true;
  };

  let open = false;
  const closeMenu = () => {
    menu.classList.add('tw-hidden');
    open = false;
  };
  const openMenu = async () => {
    menu.classList.remove('tw-hidden');
    open = true;
    await load();
  };

  button.addEventListener('click', (e) => {
    e.stopPropagation();
    if (open) closeMenu(); else openMenu();
  });
  document.addEventListener('click', (e) => {
    if (open && !el.contains(e.target as Node)) closeMenu();
  });
  el.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closeMenu();
  });
}
