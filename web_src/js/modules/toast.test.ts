import {showSuccessToast, showInfoToast, showErrorToast, showWarningToast} from './toast.ts';

test('showSuccessToast', () => {
  showSuccessToast('success', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showInfoToast', () => {
  showInfoToast('info', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showWarningToast', () => {
  showWarningToast('warning', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showErrorToast', () => {
  showErrorToast('error', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});
