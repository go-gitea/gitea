import {shouldIgnoreError} from './bootstrap.ts';

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
