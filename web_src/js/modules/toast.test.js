import {showInfoToast, showErrorToast, showWarningToast} from './toast.js';

test('showInfoToast', async () => {
  showInfoToast('success 😀', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showWarningToast', async () => {
  showWarningToast('warning 😐', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showErrorToast', async () => {
  showErrorToast('error 🙁', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});
