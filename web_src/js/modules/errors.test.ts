import {showGlobalErrorMessage, shouldIgnoreError} from './errors.ts';

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
  for (const url of [
    'https://gitea.test/assets/js/monaco.D14TzjS9.js',
    'https://gitea.test/assets/js/editor.api2.BdhK7zNg.js',
    'https://gitea.test/assets/js/editor.worker.BYgvyFya.js',
  ]) {
    const err = new Error('test');
    err.stack = `Error: test\n    at ${url}:1:1`;
    expect(shouldIgnoreError(err)).toEqual(true);
  }

  const otherError = new Error('test');
  otherError.stack = 'Error: test\n    at https://gitea.test/assets/js/index.js:1:1';
  expect(shouldIgnoreError(otherError)).toEqual(false);
});
