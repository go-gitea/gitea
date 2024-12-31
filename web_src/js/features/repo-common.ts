import {queryElems} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {sleep} from '../utils.ts';
import RepoActivityTopAuthors from '../components/RepoActivityTopAuthors.vue';
import {createApp} from 'vue';
import {toOriginUrl} from '../utils/url.ts';
import {createTippy} from '../modules/tippy.ts';

async function onDownloadArchive(e) {
  e.preventDefault();
  // there are many places using the "archive-link", eg: the dropdown on the repo code page, the release list
  const el = e.target.closest('a.archive-link[href]');
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

function initCloneSchemeUrlSelection(parent: Element) {
  const elCloneUrlInput = parent.querySelector<HTMLInputElement>('.repo-clone-url');

  const tabSsh = parent.querySelector('.repo-clone-ssh');
  const tabHttps = parent.querySelector('.repo-clone-https');
  const updateClonePanelUi = function() {
    const scheme = localStorage.getItem('repo-clone-protocol') || 'https';
    const isSSH = scheme === 'ssh' && Boolean(tabSsh) || scheme !== 'ssh' && !tabHttps;
    if (tabHttps) {
      tabHttps.textContent = window.origin.split(':')[0].toUpperCase(); // show "HTTP" or "HTTPS"
      tabHttps.classList.toggle('active', !isSSH);
    }
    if (tabSsh) {
      tabSsh.classList.toggle('active', isSSH);
    }

    const tab = isSSH ? tabSsh : tabHttps;
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
      el.href = el.getAttribute('data-href-template').replace('{url}', encodeURIComponent(link));
    }
  };

  updateClonePanelUi();
  // tabSsh or tabHttps might not both exist, eg: guest view, or one is disabled by the server
  tabSsh?.addEventListener('click', () => {
    localStorage.setItem('repo-clone-protocol', 'ssh');
    updateClonePanelUi();
  });
  tabHttps?.addEventListener('click', () => {
    localStorage.setItem('repo-clone-protocol', 'https');
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
  });
}

export function initRepoCloneButtons() {
  queryElems(document, '.js-btn-clone-panel', initClonePanelButton);
  queryElems(document, '.clone-buttons-combo', initCloneSchemeUrlSelection);
}

export async function updateIssuesMeta(url, action, issue_ids, id) {
  try {
    const response = await POST(url, {data: new URLSearchParams({action, issue_ids, id})});
    if (!response.ok) {
      throw new Error('Failed to update issues meta');
    }
  } catch (error) {
    console.error(error);
  }
}
