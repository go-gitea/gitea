import {queryElems, type DOMEvent} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {sleep} from '../utils.ts';
import RepoActivityTopAuthors from '../components/RepoActivityTopAuthors.vue';
import {createApp} from 'vue';
import {toOriginUrl} from '../utils/url.ts';
import {createTippy} from '../modules/tippy.ts';

async function onDownloadArchive(e: DOMEvent<MouseEvent>) {
  e.preventDefault();
  // there are many places using the "archive-link", eg: the dropdown on the repo code page, the release list
  const el = e.target.closest<HTMLAnchorElement>('a.archive-link[href]');
  const targetLoading = el.closest('.ui.dropdown') ?? el;
  targetLoading.classList.add('is-loading', 'loading-icon-2px');
  try {
    for (let tryCount = 0; ;tryCount++) {
      const response = await POST(el.href);
      if (!response.ok) throw new Error(`Invalid server response: ${response.status}`);

      const data = await response.json();
      if (data.complete) break;
      await sleep(Math.min((tryCount + 1) * 750, 2000));
    }
    window.location.href = el.href; // the archive is ready, start real downloading
  } catch (e) {
    console.error(e);
    showErrorToast(`Failed to download the archive: ${e}`, {duration: 2500});
  } finally {
    targetLoading.classList.remove('is-loading', 'loading-icon-2px');
  }
}

export function initRepoArchiveLinks() {
  queryElems(document, 'a.archive-link[href]', (el) => el.addEventListener('click', onDownloadArchive));
}

export function initRepoActivityTopAuthorsChart() {
  const el = document.querySelector('#repo-activity-top-authors-chart');
  if (el) {
    createApp(RepoActivityTopAuthors).mount(el);
  }
}

export function substituteRepoOpenWithUrl(tmpl: string, url: string): string {
  const pos = tmpl.indexOf('{url}');
  if (pos === -1) return tmpl;
  const posQuestionMark = tmpl.indexOf('?');
  const needEncode = posQuestionMark >= 0 && posQuestionMark < pos;
  return tmpl.replace('{url}', needEncode ? encodeURIComponent(url) : url);
}

function initCloneSchemeUrlSelection(parent: Element) {
  const elCloneUrlInput = parent.querySelector<HTMLInputElement>('.repo-clone-url');

  const tabHttps = parent.querySelector('.repo-clone-https');
  const tabSsh = parent.querySelector('.repo-clone-ssh');
  const tabTea = parent.querySelector('.repo-clone-tea');
  const updateClonePanelUi = function() {
    let scheme = localStorage.getItem('repo-clone-protocol');
    if (!['https', 'ssh', 'tea'].includes(scheme)) {
      scheme = 'https';
    }

    // Fallbacks if the scheme preference is not available in the tabs, for example: empty repo page, there are only HTTPS and SSH
    if (scheme === 'tea' && !tabTea) {
      scheme = 'https';
    }
    if (scheme === 'https' && !tabHttps) {
      scheme = 'ssh';
    } else if (scheme === 'ssh' && !tabSsh) {
      scheme = 'https';
    }

    const isHttps = scheme === 'https';
    const isSsh = scheme === 'ssh';
    const isTea = scheme === 'tea';

    if (tabHttps) {
      tabHttps.textContent = window.origin.split(':')[0].toUpperCase(); // show "HTTP" or "HTTPS"
      tabHttps.classList.toggle('active', isHttps);
    }
    if (tabSsh) {
      tabSsh.classList.toggle('active', isSsh);
    }
    if (tabTea) {
      tabTea.classList.toggle('active', isTea);
    }

    let tab: Element;
    if (isHttps) {
      tab = tabHttps;
    } else if (isSsh) {
      tab = tabSsh;
    } else if (isTea) {
      tab = tabTea;
    }

    if (!tab) return;
    const link = toOriginUrl(tab.getAttribute('data-link'));

    for (const el of document.querySelectorAll('.js-clone-url')) {
      if (el.nodeName === 'INPUT') {
        (el as HTMLInputElement).value = link;
      } else {
        el.textContent = link;
      }
    }
    for (const el of parent.querySelectorAll<HTMLAnchorElement>('.js-clone-url-editor')) {
      el.href = substituteRepoOpenWithUrl(el.getAttribute('data-href-template'), link);
    }
  };

  updateClonePanelUi();
  // tabSsh or tabHttps might not both exist, eg: guest view, or one is disabled by the server
  tabHttps?.addEventListener('click', () => {
    localStorage.setItem('repo-clone-protocol', 'https');
    updateClonePanelUi();
  });
  tabSsh?.addEventListener('click', () => {
    localStorage.setItem('repo-clone-protocol', 'ssh');
    updateClonePanelUi();
  });
  tabTea?.addEventListener('click', () => {
    localStorage.setItem('repo-clone-protocol', 'tea');
    updateClonePanelUi();
  });
  elCloneUrlInput.addEventListener('focus', () => {
    elCloneUrlInput.select();
  });
}

function initClonePanelButton(btn: HTMLButtonElement) {
  const elPanel = btn.nextElementSibling;
  // "init" must be before the "createTippy" otherwise the "tippy-target" will be removed from the document
  initCloneSchemeUrlSelection(elPanel);
  createTippy(btn, {
    content: elPanel,
    trigger: 'click',
    placement: 'bottom-end',
    interactive: true,
    hideOnClick: true,
    arrow: false,
  });
}

export function initRepoCloneButtons() {
  queryElems(document, '.js-btn-clone-panel', initClonePanelButton);
  queryElems(document, '.clone-buttons-combo', initCloneSchemeUrlSelection);
}

export async function updateIssuesMeta(url: string, action: string, issue_ids: string, id: string) {
  try {
    const response = await POST(url, {data: new URLSearchParams({action, issue_ids, id})});
    if (!response.ok) {
      throw new Error('Failed to update issues meta');
    }
  } catch (error) {
    console.error(error);
  }
}
