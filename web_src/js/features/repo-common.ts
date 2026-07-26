import {queryElems, toggleElem} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {sleep} from '../utils.ts';
import RepoActivityTopAuthors from '../components/RepoActivityTopAuthors.vue';
import {createApp} from 'vue';
import {createTippy} from '../modules/tippy.ts';
import {localUserSettings} from '../modules/user-settings.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

async function onDownloadArchive(e: Event) {
  e.preventDefault();
  // there are many places using the "archive-link", eg: the dropdown on the repo code page, the release list
  const el = (e.target as HTMLElement).closest<HTMLAnchorElement>('a.archive-link[href]')!;
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
    window.location.assign(el.href); // the archive is ready, start real downloading
  } catch (e) {
    console.error(e);
    showErrorToast(`Failed to download the archive: ${errorMessage(e)}`, {duration: 2500});
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

function initRepoCloneButtonsCombo(parent: Element) {
  // the clone section is not rendered at all when no git transport (HTTPS/SSH) is available
  const elCloneUrlInput = parent.querySelector<HTMLInputElement>('.repo-clone-url');
  if (!elCloneUrlInput) return;

  const tabHttps = parent.querySelector('.repo-clone-https');
  const tabSsh = parent.querySelector('.repo-clone-ssh');
  const tabTea = parent.querySelector('.repo-clone-tea');
  const listOpenWithEditorApps = parent.querySelector('.repo-clone-with-apps');

  // not every tab exists in every panel, eg: the admin may disable HTTP/SSH, and the empty repo page has no Tea CLI tab
  const tabByScheme: Record<string, Element | null> = {https: tabHttps, ssh: tabSsh, tea: tabTea};
  const updateClonePanelUi = function() {
    let scheme = localUserSettings.getString('repo-clone-protocol');
    // fall back to the first available tab when the preferred scheme's tab is absent (unset preference, or disabled protocol)
    if (!tabByScheme[scheme]) {
      scheme = ['https', 'ssh', 'tea'].find((s) => tabByScheme[s]) ?? '';
    }

    const isHttps = scheme === 'https';
    const isSsh = scheme === 'ssh';
    const isTea = scheme === 'tea';

    if (listOpenWithEditorApps) {
      toggleElem(listOpenWithEditorApps, !isTea); // don't show the "Open with editor apps" list when "Tea" clone is selected
    }

    if (tabHttps) {
      const link = tabHttps.getAttribute('data-link')!;
      tabHttps.textContent = link.split(':')[0].toUpperCase(); // show "HTTP" or "HTTPS"
      tabHttps.classList.toggle('active', isHttps);
    }
    if (tabSsh) {
      tabSsh.classList.toggle('active', isSsh);
    }
    if (tabTea) {
      tabTea.classList.toggle('active', isTea);
    }

    const tab = tabByScheme[scheme];
    if (!tab) return; // no protocol available at all, leave the (hidden) input untouched

    const link = tab.getAttribute('data-link')!;

    for (const el of document.querySelectorAll('.js-clone-url')) {
      if (el.nodeName === 'INPUT') {
        (el as HTMLInputElement).value = link;
      } else {
        el.textContent = link;
      }
    }
    for (const el of parent.querySelectorAll<HTMLAnchorElement>('.js-clone-url-editor')) {
      el.href = substituteRepoOpenWithUrl(el.getAttribute('data-href-template')!, link);
    }
  };

  updateClonePanelUi();
  // tabSsh or tabHttps might not both exist, eg: guest view, or one is disabled by the server
  tabHttps?.addEventListener('click', () => {
    localUserSettings.setString('repo-clone-protocol', 'https');
    updateClonePanelUi();
  });
  tabSsh?.addEventListener('click', () => {
    localUserSettings.setString('repo-clone-protocol', 'ssh');
    updateClonePanelUi();
  });
  tabTea?.addEventListener('click', () => {
    localUserSettings.setString('repo-clone-protocol', 'tea');
    updateClonePanelUi();
  });
  elCloneUrlInput.addEventListener('focus', () => {
    elCloneUrlInput.select();
  });
}

function initRepoClonePanel(btn: HTMLButtonElement) {
  const elPanel = btn.nextElementSibling!;
  // "init" must be before the "createTippy" otherwise the "tippy-target" will be removed from the document
  initRepoCloneButtonsCombo(elPanel);
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
  registerGlobalInitFunc('initRepoClonePanel', initRepoClonePanel);
  registerGlobalInitFunc('initRepoCloneButtonsCombo', initRepoCloneButtonsCombo);
}

export function sanitizeRepoName(name: string): string {
  name = name.trim().replace(/[^-.\w]/g, '-');
  for (let lastName = ''; lastName !== name;) {
    lastName = name;
    name = name.replace(/\.+$/g, '');
    name = name.replace(/\.{2,}/g, '.');
    for (const ext of ['.git', '.wiki', '.rss', '.atom']) {
      if (name.endsWith(ext)) {
        name = name.substring(0, name.length - ext.length);
      }
    }
  }
  if (['.', '..', '-'].includes(name)) name = '';
  return name;
}
