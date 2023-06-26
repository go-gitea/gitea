import {test, expect} from 'vitest';
import {showInfoToast, showErrorToast, showWarningToast} from './toast.js';

test('showInfoToast', async () => {
  await showInfoToast('success 😀', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showWarningToast', async () => {
  await showWarningToast('warning 😐', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showErrorToast', async () => {
  await showErrorToast('error 🙁', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});
