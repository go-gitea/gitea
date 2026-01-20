// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {initRepoShortcuts} from './repo-shortcuts.ts';

describe('Repository Keyboard Shortcuts', () => {
  let fileSearchInput: HTMLInputElement;
  let codeSearchInput: HTMLInputElement;
  let codeSearchHint: HTMLElement;

  beforeEach(() => {
    // Set up DOM structure
    document.body.innerHTML = `
      <div class="repo-file-search-container">
        <div class="repo-file-search-input-wrapper">
          <input type="text" placeholder="Go to file">
          <kbd class="repo-search-shortcut-hint">T</kbd>
        </div>
      </div>
      <div class="repo-home-sidebar-top">
        <div class="repo-code-search-input-wrapper">
          <input name="q" class="code-search-input" placeholder="Search code">
          <kbd class="repo-search-shortcut-hint">S</kbd>
        </div>
      </div>
    `;

    fileSearchInput = document.querySelector('.repo-file-search-container input')!;
    codeSearchInput = document.querySelector('.code-search-input')!;
    codeSearchHint = document.querySelector('.repo-code-search-input-wrapper .repo-search-shortcut-hint')!;

    initRepoShortcuts();
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  test('T key focuses file search input', () => {
    const event = new KeyboardEvent('keydown', {key: 't', bubbles: true});
    document.dispatchEvent(event);

    expect(document.activeElement).toBe(fileSearchInput);
  });

  test('Shift+T (uppercase T) focuses file search input', () => {
    const event = new KeyboardEvent('keydown', {key: 'T', bubbles: true});
    document.dispatchEvent(event);

    expect(document.activeElement).toBe(fileSearchInput);
  });

  test('S key focuses code search input', () => {
    const event = new KeyboardEvent('keydown', {key: 's', bubbles: true});
    document.dispatchEvent(event);

    expect(document.activeElement).toBe(codeSearchInput);
  });

  test('Shortcuts do not trigger when typing in input', () => {
    // Focus on an input field first
    const otherInput = document.createElement('input');
    document.body.append(otherInput);
    otherInput.focus();

    const event = new KeyboardEvent('keydown', {key: 't', bubbles: true});
    Object.defineProperty(event, 'target', {value: otherInput});
    document.dispatchEvent(event);

    // File search should not be focused because we're already in an input
    expect(document.activeElement).toBe(otherInput);
  });

  test('Shortcuts do not trigger with Ctrl modifier', () => {
    const event = new KeyboardEvent('keydown', {key: 't', ctrlKey: true, bubbles: true});
    document.dispatchEvent(event);

    expect(document.activeElement).not.toBe(fileSearchInput);
  });

  test('Shortcuts do not trigger with Meta modifier', () => {
    const event = new KeyboardEvent('keydown', {key: 's', metaKey: true, bubbles: true});
    document.dispatchEvent(event);

    expect(document.activeElement).not.toBe(codeSearchInput);
  });

  test('Shortcuts do not trigger with Alt modifier', () => {
    const event = new KeyboardEvent('keydown', {key: 't', altKey: true, bubbles: true});
    document.dispatchEvent(event);

    expect(document.activeElement).not.toBe(fileSearchInput);
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

  test('Escape key blurs code search input', () => {
    // Focus the code search input first
    codeSearchInput.focus();
    expect(document.activeElement).toBe(codeSearchInput);

    // Press Escape directly on the input (the input has its own keydown handler)
    const event = new KeyboardEvent('keydown', {key: 'Escape', bubbles: true});
    codeSearchInput.dispatchEvent(event);

    // Should no longer be focused
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
});
