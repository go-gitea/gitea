import {showGlobalErrorMessage, shouldIgnoreError} from './bootstrap.ts';

test('showGlobalErrorMessage', () => {
  document.body.innerHTML = '<div class="page-content"></div>';
  showGlobalErrorMessage('test msg 1');
  showGlobalErrorMessage('test msg 2');
  showGlobalErrorMessage('test msg 1'); // duplicated

  expect(document.body.innerHTML).toContain('>test msg 1 (2)<');
  expect(document.body.innerHTML).toContain('>test msg 2<');
  expect(document.querySelectorAll('.js-global-error').length).toEqual(2);
});

test('shouldIgnoreError', () => {
  const err = new Error('test');
  err.stack = 'Error: test\n    at https://gitea.test/assets/js/index.js:1:1';
  expect(shouldIgnoreError(err)).toEqual(false);
});
