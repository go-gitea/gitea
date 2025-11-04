import {nextTick, reactive, ref, watch} from 'vue';
import {initRepoBubbleView} from './repo-bubble-view.ts';
import {initArticleEditor} from './article-editor.ts';
import {GET} from '../modules/fetch.ts';

type ViewKey = 'bubble' | 'table' | 'article';

type RepoSelection = {
  owner: string;
  repo: string;
  subject?: string | null;
};

type HistoryState = {
  view: ViewKey;
  mode?: string;
  owner?: string | null;
  subject?: string | null;
  repo?: string | null;
};

const LS_OWNER_KEY = 'selectedArticleOwner';
const LS_SUBJECT_KEY = 'selectedArticleSubject';
const LS_REPO_KEY = 'selectedArticleRepo';

function readStoredSelection(): RepoSelection | null {
  try {
    const owner = window.localStorage.getItem(LS_OWNER_KEY);
    const repo = window.localStorage.getItem(LS_REPO_KEY);
    const subject = window.localStorage.getItem(LS_SUBJECT_KEY);
    if (!owner) return null;
    if (repo) {
      return {owner, repo, subject: subject || null};
    }
    if (!subject) return null;
    return {owner, repo: subject, subject};
  } catch {
    return null;
  }
}

function writeStoredSelection(selection: RepoSelection | null) {
  try {
    if (!selection) {
      window.localStorage.removeItem(LS_OWNER_KEY);
      window.localStorage.removeItem(LS_SUBJECT_KEY);
      window.localStorage.removeItem(LS_REPO_KEY);
      return;
    }
    window.localStorage.setItem(LS_OWNER_KEY, selection.owner);
    if (selection.subject) {
      window.localStorage.setItem(LS_SUBJECT_KEY, selection.subject);
    } else {
      window.localStorage.removeItem(LS_SUBJECT_KEY);
    }
    window.localStorage.setItem(LS_REPO_KEY, selection.repo);
  } catch {
    // ignore storage errors
  }
}

function buildSubjectUrl(base: string, view?: ViewKey): string {
  if (!view) return base;
  const url = new URL(base, window.location.origin);
  if (view === 'bubble') {
    url.searchParams.delete('view');
  } else {
    url.searchParams.set('view', view);
  }
  return url.pathname + url.search;
}

function buildSubjectUrlWithMode(base: string, view: ViewKey, mode?: string) {
  const url = new URL(buildSubjectUrl(base, view), window.location.origin);
  if (mode) url.searchParams.set('mode', mode);
  return url.pathname + url.search;
}

function buildArticleUrl(articleRepoBase: string, selection: RepoSelection, mode: string) {
  const base = articleRepoBase.replace(/\/+$/, '');
  const owner = encodeURIComponent(selection.owner);
  const repo = encodeURIComponent(selection.repo);
  const url = new URL(`${base}/${owner}/${repo}`, window.location.origin);
  url.searchParams.set('view', 'article');
  url.searchParams.set('mode', mode || 'read');
  if (selection.subject) url.searchParams.set('subject', selection.subject);
  return url.pathname + url.search;
}

function parseLocation(appSubUrl: string | undefined): HistoryState {
  const {pathname, href} = window.location;
  const url = new URL(href);
  const params = url.searchParams;
  const view = (params.get('view') as ViewKey) || 'bubble';
  const mode = params.get('mode') || 'read';

  const basePrefix = (appSubUrl || '').replace(/\/+$/, '');
  const trimmedPath = pathname.startsWith(basePrefix) ? pathname.slice(basePrefix.length) : pathname;
  const segments = trimmedPath.replace(/^\/+/, '').split('/');

  if (segments[0] === 'article') {
    if (segments[1] === 'repo' && segments.length >= 4) {
      const owner = decodeURIComponent(segments[2]);
      const repo = decodeURIComponent(segments[3]);
      const subject = params.get('subject');
      return {view: 'article', mode, owner, repo, subject};
    }
    if (segments.length >= 3) {
      const owner = decodeURIComponent(segments[1]);
      const subject = decodeURIComponent(segments[2]);
      return {view: 'article', mode, owner, subject, repo: subject};
    }
  }

  return {view, mode};
}

function matchesSelection(a: RepoSelection | null, b: RepoSelection | null) {
  if (!a || !b) return false;
  if (a.owner !== b.owner) return false;
  if (a.repo && b.repo) return a.repo === b.repo;
  return a.subject === b.subject;
}

export function initRepoHistory() {
  const root = document.querySelector<HTMLElement>('#repo-history-app');
  if (!root) return;

  const appSubUrl = window.config.appSubUrl || '';
  const subjectUrl = root.getAttribute('data-subject-url') || window.location.pathname;
  const bubbleUrl = root.getAttribute('data-bubble-url') || buildSubjectUrl(subjectUrl, 'bubble');
  const tableUrl = root.getAttribute('data-table-url') || buildSubjectUrl(subjectUrl, 'table');
  const articleRepoBase = root.getAttribute('data-article-repo-base') || `${appSubUrl}/article/repo`;

  const bubbleSection = root.querySelector<HTMLElement>('[data-view="bubble"]');
  const tableSection = root.querySelector<HTMLElement>('[data-view="table"]');
  const articleSection = root.querySelector<HTMLElement>('[data-view="article"]');

  const table = root.querySelector<HTMLTableElement>('#articles-table');

  const navEl = document.querySelector('#subject-view-tabs');

  const storedSelection = readStoredSelection();
  let initialSelection: RepoSelection | null = storedSelection;

  const initialView = root.getAttribute('data-initial-view');
  const initialOwner = root.getAttribute('data-initial-owner');
  const initialRepo = root.getAttribute('data-initial-repo');
  const initialSubject = root.getAttribute('data-initial-subject');
  const initialMode = root.getAttribute('data-initial-mode');

  if (!initialSelection && initialView === 'article' && initialOwner && (initialRepo || initialSubject)) {
    initialSelection = {
      owner: initialOwner,
      repo: initialRepo || initialSubject,
      subject: initialSubject || null,
    };
    writeStoredSelection(initialSelection);
  } else if (!initialSelection) {
    writeStoredSelection(null);
  }

  const activeView = ref<ViewKey>((initialView as ViewKey) || 'bubble');
  const articleMode = ref<string>(initialMode || 'read');
  const selectedRepo = ref<RepoSelection | null>(initialSelection);
  const isLoading = ref(false);
  const loadError = ref('');
  const articleRequestToken = ref(0);

  const viewLoaded = reactive({
    bubble: false,
    table: activeView.value === 'table',
    article: activeView.value === 'article',
  });

  let tableBound = false;
  let loaderEl: HTMLElement | null = null;
  let errorEl: HTMLElement | null = null;
  let errorTextEl: HTMLElement | null = null;
  let articleTabs: HTMLElement | null = null;
  let articleGuidance: HTMLElement | null = null;
  let articleEmptyEl: HTMLElement | null = null;
  let articleContentEl: HTMLElement | null = null;

  function collectArticleRefs() {
    if (!articleSection) return;
    loaderEl = articleSection.querySelector('[data-role="article-loader"]');
    errorEl = articleSection.querySelector('[data-role="article-error"]');
    errorTextEl = articleSection.querySelector('[data-role="article-error-text"]');
    articleTabs = articleSection.querySelector('#article-tabs');
    articleGuidance = articleSection.querySelector('#article-guidance');
    articleEmptyEl = articleSection.querySelector('[data-role="article-empty"]');
    articleContentEl = articleSection.querySelector('[data-role="article-content"]');
  }

  collectArticleRefs();
  if (!selectedRepo.value || !selectedRepo.value.repo) {
    showArticleEmpty();
  } else {
    showArticleContent();
  }
  updateArticleGuidance();

  function toggleHidden(el: Element | null, hidden: boolean) {
    if (!el) return;
    if (hidden) el.setAttribute('hidden', '');
    else el.removeAttribute('hidden');
  }

  function updateArticleGuidance() {
    if (!articleGuidance) return;
    const hasSelection = Boolean(selectedRepo.value);
    articleGuidance.style.display = hasSelection ? 'none' : '';
  }

  function showArticleEmpty() {
    toggleHidden(articleEmptyEl, false);
    toggleHidden(articleContentEl, true);
  }

  function showArticleContent() {
    toggleHidden(articleEmptyEl, true);
    toggleHidden(articleContentEl, false);
  }

  function syncNavActive() {
    if (!navEl) return;
    for (const anchor of navEl.querySelectorAll<HTMLAnchorElement>('a[data-view]')) {
      if (anchor.getAttribute('data-view') === activeView.value) {
        anchor.classList.add('active');
      } else {
        anchor.classList.remove('active');
      }
    }
  }

  function updateHistoryState(view: ViewKey, mode: string, selection: RepoSelection | null, replace = false) {
    const state: HistoryState = {
      view,
      mode,
      owner: selection?.owner ?? null,
      subject: selection?.subject ?? null,
      repo: selection?.repo ?? null,
    };

    let url: string;
    if (view === 'article' && selection) {
      url = buildArticleUrl(articleRepoBase, selection, mode);
    } else if (view === 'table') {
      url = tableUrl;
    } else if (view === 'bubble') {
      url = bubbleUrl;
    } else {
      url = buildSubjectUrlWithMode(subjectUrl, view, mode);
    }

    if (replace) {
      window.history.replaceState(state, '', url);
    } else {
      window.history.pushState(state, '', url);
    }
  }

  function updateSectionVisibility() {
    toggleHidden(bubbleSection, activeView.value !== 'bubble');
    toggleHidden(tableSection, activeView.value !== 'table');
    toggleHidden(articleSection, activeView.value !== 'article');
  }

  function updateCheckboxes() {
    if (!table) return;
    const selection = selectedRepo.value;
    for (const checkbox of table.querySelectorAll<HTMLInputElement>('tbody .row-check')) {
      const row = checkbox.closest<HTMLTableRowElement>('tr.article-row');
      if (!row) continue;
      const owner = row.getAttribute('data-owner') || '';
      const repo = row.getAttribute('data-repo') || '';
      const subject = row.getAttribute('data-subject') || '';
      checkbox.checked = Boolean(selection) && selection.owner === owner && selection.repo === (repo || subject);
    }
  }

  function updateArticleStatus() {
    if (loaderEl) toggleHidden(loaderEl, !isLoading.value);
    if (errorEl) {
      const showError = !isLoading.value && Boolean(loadError.value);
      toggleHidden(errorEl, !showError);
      if (errorTextEl) errorTextEl.textContent = loadError.value;
    }
  }

  function normalizeSelection(selection: RepoSelection | null): RepoSelection | null {
    if (!selection) return null;
    const repo = selection.repo || selection.subject || '';
    if (!selection.owner || !repo) return null;
    return {
      owner: selection.owner,
      repo,
      subject: selection.subject ?? selection.repo ?? null,
    };
  }

  function persistSelection(selection: RepoSelection | null) {
    const normalized = normalizeSelection(selection);
    if ((!selectedRepo.value && !normalized) || matchesSelection(selectedRepo.value, normalized)) {
      return;
    }
    selectedRepo.value = normalized;
    writeStoredSelection(normalized);
    window.dispatchEvent(new CustomEvent('repo:selection-updated', {detail: normalized}));
  }

  function clearSelection(pushHistory = true) {
    if (!selectedRepo.value) return;
    persistSelection(null);
    if (activeView.value === 'article') {
      switchView('bubble', {pushState: pushHistory});
    } else if (pushHistory) {
      updateHistoryState(activeView.value, articleMode.value, null, false);
    } else {
      updateHistoryState(activeView.value, articleMode.value, null, true);
    }
    if (articleSection) {
      collectArticleRefs();
      showArticleEmpty();
      updateArticleStatus();
    }
  }

  async function ensureBubbleView() {
    if (viewLoaded.bubble) return;
    await nextTick();
    initRepoBubbleView();
    viewLoaded.bubble = true;
  }

  function bindTableInteractions() {
    if (tableBound || !table) return;

    table.addEventListener('click', (event) => {
      const target = event.target as HTMLElement;
      if (!target) return;

      if (target.closest('.row-toggle')) {
        event.preventDefault();
        const btn = target.closest<HTMLButtonElement>('.row-toggle');
        if (!btn) return;
        const targetId = btn.getAttribute('data-target');
        if (!targetId) return;
        const detailRow = document.querySelector<HTMLElement>(`#${targetId}`);
        if (!detailRow) return;
        detailRow.classList.toggle('tw-hidden');
        const down = btn.querySelector<HTMLElement>('.icon-down');
        const up = btn.querySelector<HTMLElement>('.icon-up');
        const isHidden = detailRow.classList.contains('tw-hidden');
        if (down && up) {
          if (isHidden) {
            down.classList.remove('tw-hidden');
            up.classList.add('tw-hidden');
          } else {
            down.classList.add('tw-hidden');
            up.classList.remove('tw-hidden');
          }
        }
        return;
      }

      if (target.closest('.go-to-article')) {
        const btn = target.closest<HTMLButtonElement>('.go-to-article');
        if (!btn) return;
        const owner = btn.getAttribute('data-owner') || '';
        const subject = btn.getAttribute('data-subject') || '';
        const repo = btn.getAttribute('data-repo') || subject;
        if (!owner || !repo) return;
        event.preventDefault();
        switchView('article', {
          selection: {owner, subject, repo},
          mode: 'read',
          pushState: true,
        });
        return;
      }

      const row = target.closest<HTMLTableRowElement>('tr.article-row');
      if (!row) return;
      if (target.closest('input') || target.closest('label')) return;
      if (target.closest('.ui.checkbox')) return;
      const owner = row.getAttribute('data-owner') || '';
      const subject = row.getAttribute('data-subject') || '';
      const repo = row.getAttribute('data-repo') || subject;
      if (!owner || !repo) return;
      switchView('article', {
        selection: {owner, subject, repo},
        mode: 'read',
        pushState: true,
      });
    });

    table.addEventListener('change', (event) => {
      const target = event.target as HTMLInputElement;
      if (!target || target.type !== 'checkbox' || !target.classList.contains('row-check')) return;
      const row = target.closest<HTMLTableRowElement>('tr.article-row');
      if (!row) return;
      const owner = row.getAttribute('data-owner') || '';
      const subject = row.getAttribute('data-subject') || '';
      const repo = row.getAttribute('data-repo') || subject;
      if (!owner || !repo) return;
      if (target.checked) {
        for (const checkbox of table.querySelectorAll<HTMLInputElement>('tbody .row-check')) {
          if (checkbox !== target) checkbox.checked = false;
        }
        persistSelection({owner, subject, repo});
      } else if (matchesSelection(selectedRepo.value, {owner, subject, repo})) {
        persistSelection(null);
      }
    });

    tableBound = true;

    const $ = (window as unknown as {$?: any}).$;
    if ($ && typeof $.fn?.dropdown === 'function') {
      $('.ui.dropdown').dropdown();
    }
  }

  function bindArticleTabs() {
    if (!articleTabs) return;
    for (const anchor of articleTabs.querySelectorAll<HTMLAnchorElement>('a[data-article-tab]')) {
      anchor.addEventListener('click', (event) => {
        event.preventDefault();
        const tab = anchor.getAttribute('data-article-tab') || 'read';
        if (!selectedRepo.value) return;
        switchView('article', {
          selection: selectedRepo.value,
          mode: tab,
          pushState: true,
        });
      });
    }
  }

  async function loadArticleContent(selection: RepoSelection, mode: string, pushState: boolean) {
    const currentToken = ++articleRequestToken.value;
    isLoading.value = true;
    loadError.value = '';
    updateArticleStatus();
    showArticleContent();
    const url = buildArticleUrl(articleRepoBase, selection, mode);
    try {
      const response = await GET(url);
      if (!response.ok) throw new Error(`Failed with status ${response.status}`);
      const html = await response.text();
      if (articleRequestToken.value !== currentToken) return;
      const parser = new DOMParser();
      const doc = parser.parseFromString(html, 'text/html');
      const newSection = doc.querySelector('.history-view-section--article');
      if (newSection && articleSection) {
        articleSection.innerHTML = newSection.innerHTML;
        collectArticleRefs();
        showArticleContent();
        const newMode = articleSection.querySelector<HTMLElement>('#article-view-root')?.getAttribute('data-article-mode');
        articleMode.value = newMode || mode;
        bindArticleTabs();
        updateArticleGuidance();
        updateArticleStatus();
        if (articleMode.value === 'edit') {
          initArticleEditor();
        }
      }
      viewLoaded.article = true;
      isLoading.value = false;
      updateArticleStatus();
      if (pushState) {
        updateHistoryState('article', articleMode.value, selection, false);
      }
    } catch (err) {
      if (articleRequestToken.value !== currentToken) return;
      console.error('Failed to load article view', err);
      isLoading.value = false;
      loadError.value = 'Unable to load article view';
      updateArticleStatus();
    }
  }

  async function switchView(view: ViewKey, options: {
    selection?: RepoSelection;
    mode?: string;
    pushState?: boolean;
  } = {}) {
    const targetSelection = options.selection ?? selectedRepo.value;
    const nextMode = (options.mode ?? articleMode.value) || 'read';

    if (activeView.value !== view) {
      activeView.value = view;
    }

    if (articleMode.value !== nextMode) {
      articleMode.value = nextMode;
    }

    if (view === 'bubble') {
      await ensureBubbleView();
      if (options.pushState) updateHistoryState('bubble', articleMode.value, selectedRepo.value, false);
      return;
    }

    if (view === 'table') {
      bindTableInteractions();
      if (options.pushState) updateHistoryState('table', articleMode.value, selectedRepo.value, false);
      return;
    }

    if (!targetSelection || !targetSelection.repo) {
      persistSelection(null);
      viewLoaded.article = true;
      if (options.pushState) updateHistoryState('article', articleMode.value, null, false);
      showArticleEmpty();
      updateArticleStatus();
      return;
    }

    if (!matchesSelection(selectedRepo.value, targetSelection)) {
      persistSelection(targetSelection);
    }

    await loadArticleContent(targetSelection, articleMode.value, options.pushState ?? false);
  }

  function handleBubbleSelection(event: Event) {
    const rawDetail = (event as CustomEvent).detail as RepoSelection | null;
    const detail = normalizeSelection(rawDetail);
    if (!detail) {
      clearSelection(false);
      return;
    }
    if (!selectedRepo.value || !matchesSelection(selectedRepo.value, detail)) {
      persistSelection(detail);
    }
  }

  function handleBubbleOpenArticle(event: Event) {
    const rawDetail = (event as CustomEvent).detail as RepoSelection | null;
    const detail = normalizeSelection(rawDetail);
    if (!detail) return;
    if (!selectedRepo.value || !matchesSelection(selectedRepo.value, detail)) {
      persistSelection(detail);
    }
    switchView('article', {
      selection: detail,
      mode: 'read',
      pushState: true,
    });
  }

  function handleNavClick(event: MouseEvent) {
    const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>('a[data-view]');
    if (!anchor) return;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return;
    const view = anchor.getAttribute('data-view') as ViewKey;
    if (!view) return;
    event.preventDefault();
    switchView(view, {pushState: true});
  }

  function handlePopState(event: PopStateEvent) {
    const state = (event.state as HistoryState) || parseLocation(appSubUrl);
    const sel = state.owner && (state.repo || state.subject) ?
      {owner: state.owner, repo: state.repo || state.subject, subject: state.subject ?? state.repo ?? null} :
      null;
    if (!matchesSelection(selectedRepo.value, sel)) {
      persistSelection(sel);
    }
    articleMode.value = state.mode || 'read';
    switchView(state.view || 'bubble', {
      selection: sel || undefined,
      mode: articleMode.value,
      pushState: false,
    });
  }

  watch(activeView, () => {
    updateSectionVisibility();
    syncNavActive();
    if (activeView.value === 'bubble') ensureBubbleView();
    if (activeView.value === 'table') bindTableInteractions();
  }, {immediate: true});

  watch(selectedRepo, () => {
    updateCheckboxes();
    updateArticleGuidance();
  }, {immediate: true});

  watch([isLoading, loadError], () => {
    updateArticleStatus();
  }, {immediate: true});

  updateArticleGuidance();
  bindArticleTabs();
  if (articleMode.value === 'edit') {
    initArticleEditor();
  }

  const initialState: HistoryState = {
    view: activeView.value,
    mode: articleMode.value,
    owner: selectedRepo.value?.owner ?? null,
    subject: selectedRepo.value?.subject ?? null,
    repo: selectedRepo.value?.repo ?? null,
  };
  window.history.replaceState(initialState, '', window.location.pathname + window.location.search);

  window.addEventListener('repo:bubble-selected', handleBubbleSelection as EventListener);
  window.addEventListener('repo:bubble-open-article', handleBubbleOpenArticle as EventListener);
  if (navEl) navEl.addEventListener('click', handleNavClick as EventListener);
  window.addEventListener('popstate', handlePopState);
}
