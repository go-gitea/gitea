// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {initRepoCodeSearchShortcut} from './repo-shortcuts.ts';

describe('Repository Code Search Shortcut Hint', () => {
  let codeSearchInput: HTMLInputElement;
  let codeSearchHint: HTMLElement;

  beforeEach(() => {
    // Set up DOM structure for code search
    document.body.innerHTML = `
      <div class="repo-home-sidebar-top">
        <div class="repo-code-search-input-wrapper">
          <input name="q" class="code-search-input" placeholder="Search code" data-global-keyboard-shortcut="s" data-global-init="initRepoCodeSearchShortcut">
          <kbd class="repo-search-shortcut-hint">S</kbd>
        </div>
      </div>
    `;

    codeSearchInput = document.querySelector('.code-search-input')!;
    codeSearchHint = document.querySelector('.repo-code-search-input-wrapper .repo-search-shortcut-hint')!;

    // Initialize the shortcut hint functionality directly
    initRepoCodeSearchShortcut(codeSearchInput);
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  test('Code search hint hides when input has value', () => {
    // Initially visible
    expect(codeSearchHint.style.display).toBe('');

    // Type something in the code search
    codeSearchInput.value = 'test';
    codeSearchInput.dispatchEvent(new Event('input'));

    // Should be hidden
    expect(codeSearchHint.style.display).toBe('none');
  });

  test('Code search hint shows when input is cleared', () => {
    // Set a value and trigger input
    codeSearchInput.value = 'test';
    codeSearchInput.dispatchEvent(new Event('input'));
    expect(codeSearchHint.style.display).toBe('none');

    // Clear the value
    codeSearchInput.value = '';
    codeSearchInput.dispatchEvent(new Event('input'));

    // Should be visible again
    expect(codeSearchHint.style.display).toBe('');
  });

  test('Escape key clears and blurs code search input', () => {
    // Set a value and focus the input
    codeSearchInput.value = 'test';
    codeSearchInput.dispatchEvent(new Event('input'));
    codeSearchInput.focus();
    expect(document.activeElement).toBe(codeSearchInput);
    expect(codeSearchInput.value).toBe('test');

    // Press Escape directly on the input
    const event = new KeyboardEvent('keydown', {key: 'Escape', bubbles: true});
    codeSearchInput.dispatchEvent(event);

    // Value should be cleared and input should be blurred
    expect(codeSearchInput.value).toBe('');
    expect(document.activeElement).not.toBe(codeSearchInput);
  });

  test('Code search kbd hint hides on focus', () => {
    // Initially visible
    expect(codeSearchHint.style.display).toBe('');

    // Focus the input
    codeSearchInput.focus();
    codeSearchInput.dispatchEvent(new Event('focus'));

    // Should be hidden
    expect(codeSearchHint.style.display).toBe('none');

    // Blur the input
    codeSearchInput.blur();
    codeSearchInput.dispatchEvent(new Event('blur'));

    // Should be visible again
    expect(codeSearchHint.style.display).toBe('');
  });

  test('Change event also updates hint visibility', () => {
    // Initially visible
    expect(codeSearchHint.style.display).toBe('');

    // Set value via change event (e.g., browser autofill)
    codeSearchInput.value = 'autofilled';
    codeSearchInput.dispatchEvent(new Event('change'));

    // Should be hidden
    expect(codeSearchHint.style.display).toBe('none');
  });
});
