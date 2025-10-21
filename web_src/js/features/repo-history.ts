import {nextTick, reactive, ref, watch} from 'vue';
import {initRepoBubbleView} from './repo-bubble-view.ts';
import {initArticleEditor} from './article-editor.ts';

type ViewKey = 'bubble' | 'table' | 'article';

type RepoSelection = {
  owner: string;
  subject: string;
};

type HistoryState = {
  view: ViewKey;
  mode?: string;
  owner?: string | null;
  subject?: string | null;
};

const LS_OWNER_KEY = 'selectedArticleOwner';
const LS_SUBJECT_KEY = 'selectedArticleSubject';

function readStoredSelection(): RepoSelection | null {
  try {
    const owner = window.localStorage.getItem(LS_OWNER_KEY);
    const subject = window.localStorage.getItem(LS_SUBJECT_KEY);
    if (!owner || !subject) return null;
    return {owner, subject};
  } catch {
    return null;
  }
}

function writeStoredSelection(selection: RepoSelection | null) {
  try {
    if (!selection) {
      window.localStorage.removeItem(LS_OWNER_KEY);
      window.localStorage.removeItem(LS_SUBJECT_KEY);
      return;
    }
    window.localStorage.setItem(LS_OWNER_KEY, selection.owner);
    window.localStorage.setItem(LS_SUBJECT_KEY, selection.subject);
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

function buildArticleUrl(articleBase: string, selection: RepoSelection, mode: string) {
  const base = articleBase.replace(/\/+$/, '');
  const owner = encodeURIComponent(selection.owner);
  const subject = encodeURIComponent(selection.subject);
  const url = new URL(`${base}/${owner}/${subject}`, window.location.origin);
  url.searchParams.set('view', 'article');
  url.searchParams.set('mode', mode || 'read');
  return url.pathname + url.search;
}

function parseLocation(appSubUrl: string | undefined): HistoryState {
  const {pathname, search} = window.location;
  const url = new URL(window.location.href);
  const params = url.searchParams;
  const view = (params.get('view') as ViewKey) || 'bubble';
  const mode = params.get('mode') || 'read';

  const basePrefix = (appSubUrl || '').replace(/\/+$/, '');
  const trimmedPath = pathname.startsWith(basePrefix) ? pathname.slice(basePrefix.length) : pathname;
  const segments = trimmedPath.replace(/^\/+/, '').split('/');

  if (segments[0] === 'article' && segments.length >= 3) {
    const owner = decodeURIComponent(segments[1]);
    const subject = decodeURIComponent(segments[2]);
    return {view: 'article', mode, owner, subject};
  }

  return {view, mode};
}

function matchesSelection(a: RepoSelection | null, b: RepoSelection | null) {
  if (!a || !b) return false;
  return a.owner === b.owner && a.subject === b.subject;
}

export function initRepoHistory() {
  const root = document.getElementById('repo-history-app');
  if (!root) return;

  const dataset = root.dataset;
  const subjectUrl = dataset.subjectUrl || window.location.pathname;
  const bubbleUrl = dataset.bubbleUrl || buildSubjectUrl(subjectUrl, 'bubble');
  const tableUrl = dataset.tableUrl || buildSubjectUrl(subjectUrl, 'table');
  const articleBase = dataset.articleBase || `${window.APP_SUB_URL || ''}/article`;
  const appSubUrl = window.APP_SUB_URL || '';

  const bubbleSection = root.querySelector<HTMLElement>('[data-view="bubble"]');
  const tableSection = root.querySelector<HTMLElement>('[data-view="table"]');
  let articleSection = root.querySelector<HTMLElement>('[data-view="article"]');

  const table = root.querySelector<HTMLTableElement>('#articles-table');

  const navEl = document.getElementById('subject-view-tabs');

  const storedSelection = readStoredSelection();
  let initialSelection: RepoSelection | null = storedSelection;

  if (!initialSelection && dataset.initialView === 'article' && dataset.initialOwner && dataset.initialSubject) {
    initialSelection = {
      owner: dataset.initialOwner,
      subject: dataset.initialSubject,
    };
    writeStoredSelection(initialSelection);
  } else if (!initialSelection) {
    writeStoredSelection(null);
  }

  const activeView = ref<ViewKey>((dataset.initialView as ViewKey) || 'bubble');
  const articleMode = ref<string>(dataset.initialMode || 'read');
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

  function collectArticleRefs() {
    if (!articleSection) return;
    loaderEl = articleSection.querySelector('[data-role="article-loader"]');
    errorEl = articleSection.querySelector('[data-role="article-error"]');
    errorTextEl = articleSection.querySelector('[data-role="article-error-text"]');
    articleTabs = articleSection.querySelector('#article-tabs');
    articleGuidance = articleSection.querySelector('#article-guidance');
  }

  collectArticleRefs();

  function toggleHidden(el: Element | null, hidden: boolean) {
    if (!el) return;
    if (hidden) el.setAttribute('hidden', '');
    else el.removeAttribute('hidden');
  }

  function updateArticleGuidance() {
    if (!articleGuidance) return;
    const hasSelection = !!selectedRepo.value;
    articleGuidance.style.display = hasSelection ? 'none' : '';
  }

  function syncNavActive() {
    if (!navEl) return;
    navEl.querySelectorAll<HTMLAnchorElement>('a[data-view]').forEach((anchor) => {
      if (anchor.dataset.view === activeView.value) {
        anchor.classList.add('active');
      } else {
        anchor.classList.remove('active');
      }
    });
  }

  function updateHistoryState(view: ViewKey, mode: string, selection: RepoSelection | null, replace = false) {
    const state: HistoryState = {
      view,
      mode,
      owner: selection?.owner ?? null,
      subject: selection?.subject ?? null,
    };

    let url: string;
    if (view === 'article' && selection) {
      url = buildArticleUrl(articleBase, selection, mode);
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
    table.querySelectorAll<HTMLInputElement>('tbody .row-check').forEach((checkbox) => {
      const row = checkbox.closest<HTMLTableRowElement>('tr.article-row');
      if (!row) return;
      const owner = row.dataset.owner || '';
      const subject = row.dataset.subject || '';
      checkbox.checked = !!selection && selection.owner === owner && selection.subject === subject;
    });
  }

  function updateArticleStatus() {
    if (loaderEl) toggleHidden(loaderEl, !isLoading.value);
    if (errorEl) {
      const showError = !isLoading.value && !!loadError.value;
      toggleHidden(errorEl, !showError);
      if (errorTextEl) errorTextEl.textContent = loadError.value;
    }
  }

  function persistSelection(selection: RepoSelection | null) {
    if ((!selectedRepo.value && !selection) || matchesSelection(selectedRepo.value, selection)) {
      return;
    }
    const normalized = selection ? {...selection} : null;
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
        const targetId = btn.dataset.target;
        if (!targetId) return;
        const detailRow = document.getElementById(targetId);
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
        const owner = btn.dataset.owner || '';
        const subject = btn.dataset.subject || '';
        if (!owner || !subject) return;
        event.preventDefault();
        switchView('article', {
          selection: {owner, subject},
          mode: 'read',
          pushState: true,
        });
        return;
      }

      const row = target.closest<HTMLTableRowElement>('tr.article-row');
      if (!row) return;
      if (target.closest('input') || target.closest('label')) return;
      if (target.closest('.ui.checkbox')) return;
      const owner = row.dataset.owner || '';
      const subject = row.dataset.subject || '';
      if (!owner || !subject) return;
      switchView('article', {
        selection: {owner, subject},
        mode: 'read',
        pushState: true,
      });
    });

    table.addEventListener('change', (event) => {
      const target = event.target as HTMLInputElement;
      if (!target || target.type !== 'checkbox' || !target.classList.contains('row-check')) return;
      const row = target.closest<HTMLTableRowElement>('tr.article-row');
      if (!row) return;
      const owner = row.dataset.owner || '';
      const subject = row.dataset.subject || '';
      if (!owner || !subject) return;
      if (target.checked) {
        table.querySelectorAll<HTMLInputElement>('tbody .row-check').forEach((checkbox) => {
          if (checkbox !== target) checkbox.checked = false;
        });
        persistSelection({owner, subject});
      } else if (matchesSelection(selectedRepo.value, {owner, subject})) {
        persistSelection(null);
      }
    });

    tableBound = true;

    const $ = (window as unknown as { $?: any }).$;
    if ($ && typeof $.fn?.dropdown === 'function') {
      $('.ui.dropdown').dropdown();
    }
  }

  function bindArticleTabs() {
    if (!articleTabs) return;
    articleTabs.querySelectorAll<HTMLAnchorElement>('a[data-article-tab]').forEach((anchor) => {
      anchor.addEventListener('click', (event) => {
        event.preventDefault();
        const tab = anchor.dataset.articleTab || 'read';
        if (!selectedRepo.value) return;
        switchView('article', {
          selection: selectedRepo.value!,
          mode: tab,
          pushState: true,
        });
      });
    });
  }

  async function loadArticleContent(selection: RepoSelection, mode: string, pushState: boolean) {
    const currentToken = ++articleRequestToken.value;
    isLoading.value = true;
    loadError.value = '';
    updateArticleStatus();
    const url = buildArticleUrl(articleBase, selection, mode);
    try {
      const response = await fetch(url, {credentials: 'same-origin'});
      if (!response.ok) throw new Error(`Failed with status ${response.status}`);
      const html = await response.text();
      if (articleRequestToken.value !== currentToken) return;
      const parser = new DOMParser();
      const doc = parser.parseFromString(html, 'text/html');
      const newSection = doc.querySelector('.history-view-section--article');
      if (newSection && articleSection) {
        articleSection.innerHTML = newSection.innerHTML;
        collectArticleRefs();
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

    if (!targetSelection) {
      persistSelection(null);
      viewLoaded.article = true;
      if (options.pushState) updateHistoryState('article', articleMode.value, null, false);
      return;
    }

    if (!matchesSelection(selectedRepo.value, targetSelection)) {
      persistSelection(targetSelection);
    }

    await loadArticleContent(targetSelection, articleMode.value, options.pushState ?? false);
  }

  function handleBubbleSelection(event: Event) {
    const detail = (event as CustomEvent).detail as RepoSelection | null;
    if (!detail) {
      clearSelection(false);
      return;
    }
    if (!selectedRepo.value || !matchesSelection(selectedRepo.value, detail)) {
      persistSelection(detail);
    }
  }

  function handleBubbleOpenArticle(event: Event) {
    const detail = (event as CustomEvent).detail as RepoSelection | null;
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
    const view = anchor.dataset.view as ViewKey;
    if (!view) return;
    event.preventDefault();
    switchView(view, {pushState: true});
  }

  function handlePopState(event: PopStateEvent) {
    const state = (event.state as HistoryState) || parseLocation(appSubUrl);
    const sel = state.owner && state.subject ? {owner: state.owner, subject: state.subject} : null;
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
  };
  window.history.replaceState(initialState, '', window.location.pathname + window.location.search);

  window.addEventListener('repo:bubble-selected', handleBubbleSelection as EventListener);
  window.addEventListener('repo:bubble-open-article', handleBubbleOpenArticle as EventListener);
  if (navEl) navEl.addEventListener('click', handleNavClick);
  window.addEventListener('popstate', handlePopState);
}
