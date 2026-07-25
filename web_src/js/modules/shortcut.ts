import {registerGlobalInitFunc} from './observer.ts';
import {hideElem, toggleElem} from '../utils/dom.ts';

function initShortcutKbd(kbd: HTMLElement) {
  // Handle initial state: hide the kbd hint if the associated input already has a value
  // (e.g., from browser autofill or back/forward navigation cache)
  const elem = elemFromKbd(kbd);
  if (elem?.value) hideElem(kbd);
  kbd.setAttribute('aria-hidden', 'true');
  kbd.setAttribute('aria-keyshortcuts', kbd.getAttribute('data-shortcut-keys')!);
}

function shortcutWrapper(el: HTMLElement): HTMLElement | null {
  const parent = el.parentElement;
  return parent?.matches('.global-shortcut-wrapper') ? parent : null;
}

function elemFromKbd(kbd: HTMLElement): HTMLInputElement | HTMLTextAreaElement | null {
  return shortcutWrapper(kbd)?.querySelector<HTMLInputElement>('input, textarea') || null;
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
  if (window.config?.pageData?.repoLink) {
    return window.config.pageData.repoLink;
  }
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

function handleSequenceShortcut(key: string) {
  // If no sequence key is set, check if this is a sequence starter
  if (!sequenceKey) {
    if (key === 'g') {
      sequenceKey = 'g';
      sequenceTimeout = window.setTimeout(resetSequence, SEQUENCE_TIMEOUT_MS);
      return true;
    }
    return false;
  }

  // Handle g + key sequences
  if (sequenceKey === 'g') {
    resetSequence();
    const appSubUrl = window.config?.appSubUrl || '';
    const repoLink = getRepoLink();

    switch (key) {
      case 'i':
        window.location.assign(repoLink ? `${repoLink}/issues` : `${appSubUrl}/issues`);
        return true;
      case 'p':
        window.location.assign(repoLink ? `${repoLink}/pulls` : `${appSubUrl}/pulls`);
        return true;
      case 'c':
        if (repoLink) {
          window.location.assign(repoLink);
          return true;
        }
        return false;
      case 'd':
        window.location.assign(`${appSubUrl}/`);
        return true;
    }
  }

  resetSequence();
  return false;
}

export function initGlobalShortcut() {
  registerGlobalInitFunc('onGlobalShortcut', initShortcutKbd);

  const helpModal = document.querySelector<HTMLElement>('#keyboard-shortcuts-modal');
  if (helpModal) {
    const closeModal = () => helpModal.classList.add('hidden');
    helpModal.querySelector('.close-button')?.addEventListener('click', closeModal);
    helpModal.querySelector('.ui.button.primary')?.addEventListener('click', closeModal);
    helpModal.querySelector('.modal-backdrop')?.addEventListener('click', closeModal);

    document.querySelector('#show-keyboard-shortcuts')?.addEventListener('click', (e) => {
      e.preventDefault();
      helpModal.classList.remove('hidden');
    });
  }

  // A <kbd> element next to an <input> declares a keyboard shortcut for that input.
  // When the matching key is pressed, the sibling input is focused.
  // When Escape is pressed inside such an input, the input is cleared and blurred.
  // The <kbd> element is shown/hidden automatically based on input focus and value.
  document.addEventListener('keydown', (e: KeyboardEvent) => {
    // Modifier keys are not supported yet
    if (e.ctrlKey || e.metaKey || e.altKey) return;

    const target = e.target as HTMLElement;

    // Handle Escape: clear and blur inputs that have an associated keyboard shortcut, and close modal
    if (e.key === 'Escape') {
      const helpModal = document.querySelector<HTMLElement>('#keyboard-shortcuts-modal');
      if (helpModal && !helpModal.classList.contains('hidden')) {
        helpModal.classList.add('hidden');
      }

      const kbd = kbdFromElem(target);
      if (kbd) {
        (target as HTMLInputElement).value = '';
        (target as HTMLInputElement).blur();
      }
      resetSequence();
      return;
    }

    // Don't trigger shortcuts when typing in input fields or contenteditable areas
    if (target.matches('input, textarea, select') || target.isContentEditable) {
      resetSequence();
      return;
    }

    const key = e.key.toLowerCase();

    // Handle help modal trigger (?)
    if (key === '?') {
      e.preventDefault();
      const helpModal = document.querySelector<HTMLElement>('#keyboard-shortcuts-modal');
      if (helpModal) {
        helpModal.classList.toggle('hidden');
      }
      resetSequence();
      return;
    }

    // Handle specific page shortcuts
    if (key === 'w') {
      const branchDropdown = document.querySelector<HTMLElement>('.branch-dropdown-button');
      if (branchDropdown) {
        e.preventDefault();
        branchDropdown.click();
        return;
      }
    } else if (key === 'y') {
      const permalinkBtn = document.querySelector<HTMLAnchorElement>('#file-permalink-button');
      if (permalinkBtn) {
        e.preventDefault();
        window.location.assign(permalinkBtn.href);
        return;
      }
    } else if (key === 'b') {
      const blameBtn = document.querySelector<HTMLAnchorElement>('#file-blame-button');
      if (blameBtn) {
        e.preventDefault();
        window.location.assign(blameBtn.href);
        return;
      }
    }

    // Handle sequence shortcuts (g + key)
    if (handleSequenceShortcut(key)) {
      e.preventDefault();
      return;
    }

    // Find kbd element with matching shortcut (case-insensitive), then focus its sibling input
    // At the moment, only a simple match. In the future, it can be extended to support modifiers and key combinations
    const kbd = document.querySelector<HTMLElement>(`.global-shortcut-wrapper > kbd[data-shortcut-keys="${CSS.escape(key)}"]`);
    if (!kbd) return;
    e.preventDefault();
    elemFromKbd(kbd)!.focus();
  });

  // Toggle kbd shortcut hint visibility on input focus/blur
  document.addEventListener('focusin', (e) => {
    const kbd = kbdFromElem(e.target as HTMLElement);
    if (!kbd) return;
    hideElem(kbd);
  });

  document.addEventListener('focusout', (e) => {
    const kbd = kbdFromElem(e.target as HTMLElement);
    if (!kbd) return;
    const hasContent = Boolean((e.target as HTMLInputElement).value);
    toggleElem(kbd, !hasContent);
  });
}
