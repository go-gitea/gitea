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
  return tmpl.replace('{url}', () => needEncode ? encodeURIComponent(url) : url);
}

function initRepoCloneButtonsCombo(parent: Element, onPanelChange?: () => void) {
  // the clone section is not rendered at all when no git transport (HTTPS/SSH) is available
  const elCloneUrlInput = parent.querySelector<HTMLInputElement>('.repo-clone-url');
  if (!elCloneUrlInput) return;

  const listOpenWithEditorApps = parent.querySelector('.repo-clone-with-apps');
  // not every tab exists in every panel, eg: the admin may disable HTTP/SSH, and the empty repo page has no Gitea CLI tab
  const tabByScheme: Record<string, Element | null> = {
    https: parent.querySelector('.repo-clone-https'),
    ssh: parent.querySelector('.repo-clone-ssh'),
    cli: parent.querySelector('.repo-clone-cli'),
  };
  if (tabByScheme.https) {
    tabByScheme.https.textContent = tabByScheme.https.getAttribute('data-link')!.split(':')[0].toUpperCase(); // show "HTTP" or "HTTPS"
  }
  const updateClonePanelUi = function() {
    let scheme = localUserSettings.getString('repo-clone-protocol');
    // fall back to the first available tab when the preferred scheme's tab is absent (unset preference, or disabled protocol)
    if (!tabByScheme[scheme]) {
      scheme = Object.keys(tabByScheme).find((s) => tabByScheme[s])!;
    }

    if (listOpenWithEditorApps) {
      toggleElem(listOpenWithEditorApps, scheme !== 'cli'); // editor links need a Git URL, not a CLI command
    }
    for (const [candidate, tab] of Object.entries(tabByScheme)) {
      tab?.classList.toggle('active', candidate === scheme);
    }

    const link = tabByScheme[scheme]!.getAttribute('data-link')!;
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
  for (const [scheme, tab] of Object.entries(tabByScheme)) {
    tab?.addEventListener('click', () => {
      localUserSettings.setString('repo-clone-protocol', scheme);
      updateClonePanelUi();
      onPanelChange?.();
    });
  }
  elCloneUrlInput.addEventListener('focus', () => {
    elCloneUrlInput.select();
  });
}

export function initRepoClonePanel(btn: HTMLButtonElement) {
  const elPanel = btn.nextElementSibling!;
  let tippyInstance: ReturnType<typeof createTippy>;
  const observer = new IntersectionObserver(([entry]) => {
    if (!entry.isIntersecting) tippyInstance.hide();
  });
  // "init" must be before the "createTippy" otherwise the "tippy-target" will be removed from the document
  initRepoCloneButtonsCombo(elPanel, () => tippyInstance.popperInstance!.update());
  tippyInstance = createTippy(btn, {
    content: elPanel,
    trigger: 'click',
    placement: 'bottom-end',
    interactive: true,
    hideOnClick: true,
    arrow: false,
    limitSizeToViewport: {vertical: true},
    onMount: () => observer.observe(btn),
    onHidden: () => observer.disconnect(),
    onDestroy: () => observer.disconnect(),
  });

  const cloneTab = elPanel.querySelector<HTMLButtonElement>('.repo-clone-main');
  if (cloneTab) {
    const codespacesTab = elPanel.querySelector<HTMLButtonElement>('.repo-codespaces-main')!;
    const cloneSection = elPanel.querySelector<HTMLElement>('.repo-clone-section')!;
    const codespacesSection = elPanel.querySelector<HTMLElement>('.repo-codespaces-section')!;
    const showClone = (visible: boolean) => {
      cloneTab.classList.toggle('active', visible);
      codespacesTab.classList.toggle('active', !visible);
      cloneTab.setAttribute('aria-selected', String(visible));
      codespacesTab.setAttribute('aria-selected', String(!visible));
      cloneTab.tabIndex = visible ? 0 : -1;
      codespacesTab.tabIndex = visible ? -1 : 0;
      cloneSection.hidden = !visible;
      codespacesSection.hidden = visible;
      tippyInstance.popperInstance!.update();
    };
    cloneTab.addEventListener('click', () => showClone(true));
    codespacesTab.addEventListener('click', () => showClone(false));
    cloneTab.addEventListener('keydown', (event) => {
      if (event.key !== 'ArrowRight' || event.isComposing) return;
      event.preventDefault();
      codespacesTab.click();
      codespacesTab.focus();
    });
    codespacesTab.addEventListener('keydown', (event) => {
      if (event.key !== 'ArrowLeft' || event.isComposing) return;
      event.preventDefault();
      cloneTab.click();
      cloneTab.focus();
    });
  }
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
