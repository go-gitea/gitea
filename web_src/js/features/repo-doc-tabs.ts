import {GET} from '../modules/fetch.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';

const fetchPromiseCache = new Map<string, Promise<string>>();

async function fetchContent(url: string): Promise<string> {
  const originUrl = new URL(url, window.location.origin);
  originUrl.searchParams.set('only_content', 'true');
  const href = originUrl.href;

  const cached = fetchPromiseCache.get(href);
  if (cached) return cached;

  const promise = (async () => {
    try {
      const resp = await GET(href);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      return await resp.text();
    } catch (err) {
      fetchPromiseCache.delete(href); // allow a retry on the next hover/click
      throw err;
    }
  })();
  fetchPromiseCache.set(href, promise);
  return promise;
}

async function prefetchContent(url: string) {
  try {
    await fetchContent(url);
  } catch {
    // ignore, a click will fall back to a normal navigation
  }
}

async function openDocTab(url: string) {
  const elContent = document.querySelector('.repo-view-content');
  if (!elContent) {
    window.location.assign(url);
    return;
  }

  try {
    elContent.innerHTML = await fetchContent(url);
    window.history.replaceState(null, '', url);
  } catch {
    window.location.assign(url); // fall back to a normal navigation
  }
}

function onRepoDocTabClick(link: HTMLAnchorElement, e: MouseEvent) {
  if (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return; // let modified clicks open a new tab
  e.preventDefault();
  if (link.classList.contains('active')) return;
  openDocTab(link.href);
}

function initRepoDocTabsPrefetch(menu: HTMLElement) {
  menu.addEventListener('pointerover', (e) => {
    const link = (e.target as Element).closest<HTMLAnchorElement>('a.item');
    if (link) prefetchContent(link.href);
  });
}

export function initRepoDocTabs() {
  registerGlobalEventFunc('click', 'onRepoDocTabClick', onRepoDocTabClick);
  registerGlobalInitFunc('initRepoDocTabsPrefetch', initRepoDocTabsPrefetch);
}
