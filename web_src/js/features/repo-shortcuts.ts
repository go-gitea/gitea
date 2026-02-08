// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {registerGlobalInitFunc} from '../modules/observer.ts';

/**
 * Initialize the code search input with shortcut hint visibility management.
 * The shortcut hint is hidden when the input has a value or is focused.
 * Pressing Escape clears the input and blurs it.
 */
export function initRepoCodeSearchShortcut(el: HTMLInputElement): void {
  const shortcutHint = el.parentElement?.querySelector<HTMLElement>('.repo-search-shortcut-hint');
  if (!shortcutHint) return;

  let isFocused = false;

  const updateHintVisibility = () => {
    shortcutHint.style.display = (el.value || isFocused) ? 'none' : '';
  };

  // Check initial value (e.g., from browser autofill or back navigation)
  updateHintVisibility();

  el.addEventListener('input', updateHintVisibility);
  el.addEventListener('change', updateHintVisibility);
  el.addEventListener('focus', () => {
    isFocused = true;
    updateHintVisibility();
  });
  el.addEventListener('blur', () => {
    isFocused = false;
    updateHintVisibility();
  });

  // Handle Escape key to clear and blur the code search input
  el.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      el.value = '';
      updateHintVisibility();
      el.blur();
    }
  });
}

export function initRepoShortcuts(): void {
  registerGlobalInitFunc('initRepoCodeSearchShortcut', initRepoCodeSearchShortcut);
}
