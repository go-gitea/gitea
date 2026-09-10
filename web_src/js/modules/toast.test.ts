import {showSuccessToast, showInfoToast, showErrorToast, showWarningToast} from './toast.ts';

test('showSuccessToast', async () => {
  showSuccessToast('success', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showInfoToast', async () => {
  showInfoToast('info', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showWarningToast', async () => {
  showWarningToast('warning', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showErrorToast', async () => {
  showErrorToast('error', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});
