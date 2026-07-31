import {registerGlobalInitFunc} from './observer.ts';
import {hideElem, toggleElem} from '../utils/dom.ts';

function initShortcutKbd(kbd: HTMLElement) {
  // Handle initial state: hide the kbd hint if the associated input already has a value
  // (e.g., from browser autofill or back/forward navigation cache)
  const elem = elemFromKbd(kbd);
  if (elem && ('value' in elem) && (elem as HTMLInputElement).value) hideElem(kbd);
  kbd.setAttribute('aria-hidden', 'true');
  kbd.setAttribute('aria-keyshortcuts', kbd.getAttribute('data-shortcut-keys')!);
}

function shortcutWrapper(el: HTMLElement): HTMLElement | null {
  const parent = el.parentElement;
  return parent?.matches('.global-shortcut-wrapper') ? parent : null;
}

function elemFromKbd(kbd: HTMLElement): HTMLElement | null {
  return shortcutWrapper(kbd)?.querySelector<HTMLElement>('input, textarea, select, a, button, .button') || null;
}

function kbdFromElem(input: HTMLElement): HTMLElement | null {
  return shortcutWrapper(input)?.querySelector<HTMLElement>('kbd') || null;
}

// Global navigation shortcuts (sequence shortcuts like g + i)
let sequenceKey: string | null = null;
let sequenceTimeout: number | null = null;

const SEQUENCE_TIMEOUT_MS = 1000;

function resetSequence() {
  sequenceKey = null;
  if (sequenceTimeout !== null) {
    clearTimeout(sequenceTimeout);
    sequenceTimeout = null;
  }
}

function getRepoLink(): string | null {
  const issuesTab = document.querySelector<HTMLAnchorElement>('.secondary-nav a[href$="/issues"]');
  if (issuesTab) {
    return issuesTab.getAttribute('href')!.replace(/\/issues$/, '');
  }
  const pullsTab = document.querySelector<HTMLAnchorElement>('.secondary-nav a[href$="/pulls"]');
  if (pullsTab) {
    return pullsTab.getAttribute('href')!.replace(/\/pulls$/, '');
  }
  return null;
}

// Only navigate to a repo unit whose tab is present (unit enabled + readable)
function hasRepoNavTab(hrefSuffix: string): boolean {
  return document.querySelector(`.secondary-nav a[href$="${CSS.escape(hrefSuffix)}"]`) !== null;
}

type RouteConfig = {
  suffix: string;
  repoOnly?: boolean;
  globalFallback?: boolean;
};

const NAVIGATION_ROUTES: Record<string, RouteConfig> = {
  i: {suffix: '/issues', globalFallback: true},
  p: {suffix: '/pulls', globalFallback: true},
  c: {suffix: '', repoOnly: true},
  a: {suffix: '/actions', repoOnly: true},
  b: {suffix: '/projects', repoOnly: true},
  w: {suffix: '/wiki', repoOnly: true},
  d: {suffix: '/', globalFallback: true},
};

function handleSequenceShortcut(key: string): boolean {
  // Handle sequence initialization or restart
  if (key === 'g') {
    resetSequence();
    sequenceKey = 'g';
    sequenceTimeout = window.setTimeout(resetSequence, SEQUENCE_TIMEOUT_MS);
    return true;
  }

  // If a sequence was active, process the second key
  if (sequenceKey === 'g') {
    resetSequence();
    const appSubUrl = window.config?.appSubUrl || '';
    const repoLink = getRepoLink();

    const route = NAVIGATION_ROUTES[key];
    if (route) {
      if (repoLink) {
        if (!route.suffix || hasRepoNavTab(route.suffix)) {
          window.location.assign(repoLink + route.suffix);
          return true;
        }
        if (route.globalFallback) {
          window.location.assign(appSubUrl + route.suffix);
          return true;
        }
      } else if (!route.repoOnly) {
        window.location.assign(appSubUrl + route.suffix);
        return true;
      }
    }
  }

  resetSequence();
  return false;
}

export function initGlobalShortcut() {
  registerGlobalInitFunc('onGlobalShortcut', initShortcutKbd);

  const helpModal = document.querySelector<HTMLElement>('#keyboard-shortcuts-modal');
  const openTrigger = document.querySelector<HTMLElement>('#show-keyboard-shortcuts');
  let lastActiveElement: HTMLElement | null = null;

  const openModal = () => {
    if (!helpModal || !helpModal.classList.contains('hidden')) return;
    lastActiveElement = document.activeElement as HTMLElement;
    helpModal.classList.remove('hidden');
  };

  const closeModal = () => {
    if (!helpModal || helpModal.classList.contains('hidden')) return;
    helpModal.classList.add('hidden');
    if (lastActiveElement && typeof lastActiveElement.focus === 'function') {
      lastActiveElement.focus({preventScroll: true});
    }
    lastActiveElement = null;
  };

  if (helpModal) {
    helpModal.querySelector('.close-button')?.addEventListener('click', closeModal);
    helpModal.querySelector('.ui.button.primary')?.addEventListener('click', closeModal);
    helpModal.querySelector('.modal-backdrop')?.addEventListener('click', closeModal);

    openTrigger?.addEventListener('click', (e) => {
      e.preventDefault();
      openModal();
    });
  }

  // A <kbd> element next to an element declares a keyboard shortcut for that element.
  // When the matching key is pressed, inputs are focused and buttons/links are clicked.
  // When Escape is pressed inside an input, the input is cleared and blurred.
  document.addEventListener('keydown', (e: KeyboardEvent) => {
    // Modifier keys are not supported yet
    if (e.ctrlKey || e.metaKey || e.altKey) return;

    const target = e.target as HTMLElement;

    // Handle Escape: clear and blur inputs that have an associated keyboard shortcut, and close modal
    if (e.key === 'Escape') {
      closeModal();

      const kbd = kbdFromElem(target);
      if (kbd) {
        (target as HTMLInputElement).value = '';
        (target as HTMLInputElement).blur();
      }
      resetSequence();
      return;
    }

    // Don't trigger shortcuts when typing in input fields or contenteditable areas
    if (target instanceof Element && (target.matches('input, textarea, select') || target.isContentEditable)) {
      resetSequence();
      return;
    }

    const key = e.key.toLowerCase();

    // Handle help modal trigger (?)
    if (key === '?') {
      e.preventDefault();
      if (helpModal) {
        if (helpModal.classList.contains('hidden')) {
          openModal();
        } else {
          closeModal();
        }
      }
      resetSequence();
      return;
    }

    // Handle sequence shortcuts (g + key)
    if (handleSequenceShortcut(key)) {
      e.preventDefault();
      return;
    }

    // Find kbd element with matching shortcut (case-insensitive), then focus or click its target element
    const kbd = document.querySelector<HTMLElement>(`.global-shortcut-wrapper > kbd[data-shortcut-keys="${CSS.escape(key)}"]`);
    if (!kbd) return;

    const elem = elemFromKbd(kbd);
    if (!elem) return;

    e.preventDefault();
    if (elem.matches('input, textarea, select')) {
      elem.focus();
    } else {
      elem.click();
    }
  });

  // Toggle kbd shortcut hint visibility on input focus/blur
  document.addEventListener('focusin', (e) => {
    const target = e.target as HTMLElement;
    if (target.matches('input, textarea')) {
      const kbd = kbdFromElem(target);
      if (kbd) hideElem(kbd);
    }
  });

  document.addEventListener('focusout', (e) => {
    const target = e.target as HTMLElement;
    if (target.matches('input, textarea')) {
      const kbd = kbdFromElem(target);
      if (!kbd) return;
      const hasContent = Boolean((target as HTMLInputElement).value);
      toggleElem(kbd, !hasContent);
    }
  });
}
