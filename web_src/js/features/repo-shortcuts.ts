// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

/**
 * Initialize global keyboard shortcuts for repository pages.
 * - 'T' key: Focus the "Go to file" search input
 * - 'S' key: Focus the "Search code" input
 *
 * Shortcuts are disabled when the user is typing in an input field,
 * textarea, or contenteditable element.
 */
export function initRepoShortcuts(): void {
  // Initialize keyboard shortcut listeners
  document.addEventListener('keydown', (e: KeyboardEvent) => {
    // Don't trigger shortcuts when typing in input fields
    const target = e.target as HTMLElement;
    if (target instanceof HTMLElement && target.matches('input, textarea, select, [contenteditable="true"]')) {
      return;
    }

    // Don't trigger shortcuts when modifier keys are pressed
    if (e.ctrlKey || e.metaKey || e.altKey) {
      return;
    }

    if (e.key === 't' || e.key === 'T') {
      const fileSearchInput = document.querySelector<HTMLInputElement>('.repo-file-search-container input');
      if (fileSearchInput) {
        e.preventDefault();
        fileSearchInput.focus();
      }
    } else if (e.key === 's' || e.key === 'S') {
      const codeSearchInput = document.querySelector<HTMLInputElement>('.repo-home-sidebar-top input[name="q"], .code-search-input');
      if (codeSearchInput) {
        e.preventDefault();
        codeSearchInput.focus();
      }
    }
  });

  // Toggle shortcut hint visibility for code search input based on input value and focus state
  const codeSearchInput = document.querySelector<HTMLInputElement>('.code-search-input');
  if (codeSearchInput) {
    const shortcutHint = codeSearchInput.parentElement?.querySelector<HTMLElement>('.repo-search-shortcut-hint');
    if (shortcutHint) {
      let isFocused = false;

      const updateHintVisibility = () => {
        shortcutHint.style.display = (codeSearchInput.value || isFocused) ? 'none' : '';
      };

      // Check initial value (e.g., from browser autofill or back navigation)
      updateHintVisibility();

      codeSearchInput.addEventListener('input', updateHintVisibility);
      codeSearchInput.addEventListener('change', updateHintVisibility);
      codeSearchInput.addEventListener('focus', () => {
        isFocused = true;
        updateHintVisibility();
      });
      codeSearchInput.addEventListener('blur', () => {
        isFocused = false;
        updateHintVisibility();
      });

      // Handle Escape key to clear and blur the code search input
      codeSearchInput.addEventListener('keydown', (e: KeyboardEvent) => {
        if (e.key === 'Escape') {
          codeSearchInput.value = '';
          updateHintVisibility();
          codeSearchInput.blur();
        }
      });
    }
  }
}
