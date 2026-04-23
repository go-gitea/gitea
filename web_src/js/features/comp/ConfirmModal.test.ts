// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {createConfirmModal} from './ConfirmModal.ts';

test('createConfirmModal with default options', () => {
  const modal = createConfirmModal();

  // Check basic structure
  expect(modal).toBeTruthy();
  expect(modal.classList.contains('ui')).toBe(true);
  expect(modal.classList.contains('g-modal-confirm')).toBe(true);
  expect(modal.classList.contains('modal')).toBe(true);

  // Check content div exists
  const content = modal.querySelector('.content');
  expect(content).toBeTruthy();
  expect(content?.textContent).toBe('');

  // Check actions div exists with buttons
  const actions = modal.querySelector('.actions');
  expect(actions).toBeTruthy();

  const cancelButton = actions?.querySelector('.cancel.button');
  const confirmButton = actions?.querySelector('.ok.button');
  expect(cancelButton).toBeTruthy();
  expect(confirmButton).toBeTruthy();

  // Default confirm button should be primary color
  expect(confirmButton?.classList.contains('primary')).toBe(true);
});

test('createConfirmModal with header and content', () => {
  const modal = createConfirmModal({
    header: 'Delete Item',
    content: 'Are you sure you want to delete this item?',
  });

  // Check header
  const header = modal.querySelector('.header');
  expect(header).toBeTruthy();
  expect(header?.textContent).toBe('Delete Item');

  // Check content
  const content = modal.querySelector('.content');
  expect(content).toBeTruthy();
  expect(content?.textContent).toBe('Are you sure you want to delete this item?');
});

test('createConfirmModal with red confirm button for risky actions', () => {
  const modal = createConfirmModal({
    header: 'Delete Item',
    content: 'This action cannot be undone.',
    confirmButtonColor: 'red',
  });

  const confirmButton = modal.querySelector('.ok.button');
  expect(confirmButton).toBeTruthy();
  expect(confirmButton?.classList.contains('red')).toBe(true);
  expect(confirmButton?.classList.contains('primary')).toBe(false);
});

test('createConfirmModal with green confirm button', () => {
  const modal = createConfirmModal({
    header: 'Confirm Action',
    content: 'Proceed with action?',
    confirmButtonColor: 'green',
  });

  const confirmButton = modal.querySelector('.ok.button');
  expect(confirmButton).toBeTruthy();
  expect(confirmButton?.classList.contains('green')).toBe(true);
});

test('createConfirmModal with blue confirm button', () => {
  const modal = createConfirmModal({
    header: 'Info',
    content: 'Some information',
    confirmButtonColor: 'blue',
  });

  const confirmButton = modal.querySelector('.ok.button');
  expect(confirmButton).toBeTruthy();
  expect(confirmButton?.classList.contains('blue')).toBe(true);
});

test('createConfirmModal buttons have correct icons', () => {
  const modal = createConfirmModal();

  const cancelButton = modal.querySelector('.cancel.button');
  const confirmButton = modal.querySelector('.ok.button');

  // Both buttons should contain SVG icons
  expect(cancelButton?.querySelector('svg')).toBeTruthy();
  expect(confirmButton?.querySelector('svg')).toBeTruthy();
});

test('createConfirmModal empty header does not create header element', () => {
  const modal = createConfirmModal({
    header: '',
    content: 'Some content',
  });

  // Empty header should not create a header div
  const header = modal.querySelector('.header');
  expect(header).toBeFalsy();
});

test('createConfirmModal structure matches expected HTML', () => {
  const modal = createConfirmModal({
    header: 'Test Header',
    content: 'Test Content',
    confirmButtonColor: 'red',
  });

  // Verify the modal structure
  expect(modal.tagName).toBe('DIV');
  expect(modal.className).toContain('ui');
  expect(modal.className).toContain('g-modal-confirm');
  expect(modal.className).toContain('modal');

  // Verify children order: header, content, actions
  const children = Array.from(modal.children);
  expect(children.length).toBe(3);

  expect(children[0].classList.contains('header')).toBe(true);
  expect(children[1].classList.contains('content')).toBe(true);
  expect(children[2].classList.contains('actions')).toBe(true);
});
