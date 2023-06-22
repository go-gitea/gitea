import {test, expect} from 'vitest';
import {showInfo, showError} from './toast.js';

test('showInfo', async () => {
  await showInfo('success :)', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});

test('showError', async () => {
  await showError('error :(', {duration: -1});
  expect(document.querySelector('.toastify')).toBeTruthy();
});
