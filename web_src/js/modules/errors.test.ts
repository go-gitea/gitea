import {isGiteaError, processWindowErrorEvent, showGlobalErrorMessage} from './errors.ts';

beforeEach(() => {
  document.body.innerHTML = '<div class="page-content"></div>';
});

test('isGiteaError', () => {
  expect(isGiteaError('', '')).toBe(true);
  expect(isGiteaError('moz-extension://abc/content.js', '')).toBe(false);
  expect(isGiteaError('safari-extension://abc/content.js', '')).toBe(false);
  expect(isGiteaError('safari-web-extension://abc/content.js', '')).toBe(false);
  expect(isGiteaError('chrome-extension://abc/content.js', '')).toBe(false);
  expect(isGiteaError('https://other-site.com/script.js', '')).toBe(false);
  expect(isGiteaError('http://localhost:3000/some/page', '')).toBe(true);
  expect(isGiteaError('http://localhost:3000/assets/js/index.abc123.js', '')).toBe(true);
  expect(isGiteaError('', `Error\n    at chrome-extension://abc/content.js:1:1`)).toBe(false);
  expect(isGiteaError('', `Error\n    at https://other-site.com/script.js:1:1`)).toBe(false);
  expect(isGiteaError('', `Error\n    at http://localhost:3000/assets/js/index.abc123.js:1:1`)).toBe(true);
  expect(isGiteaError('http://localhost:3000/assets/js/index.js', `Error\n    at chrome-extension://abc/content.js:1:1`)).toBe(false);
});

test('showGlobalErrorMessage', () => {
  showGlobalErrorMessage('test msg 1');
  showGlobalErrorMessage('test msg 2');
  showGlobalErrorMessage('test msg 1'); // duplicated

  expect(document.body.innerHTML).toContain('>test msg 1 (2)<');
  expect(document.body.innerHTML).toContain('>test msg 2<');
  expect(document.querySelectorAll('.js-global-error').length).toEqual(2);
});

test('processWindowErrorEvent renders stack trace in details', () => {
  const error = new Error('boom');
  error.stack = `Error: boom\n    at fn (${window.location.origin}/assets/js/index.js:1:1)`;
  processWindowErrorEvent({error, type: 'error'} as ErrorEvent & PromiseRejectionEvent);
  expect(document.querySelector('.js-global-error summary')!.textContent).toContain('JavaScript error: boom');
  expect(document.querySelector('.js-global-error pre')!.textContent).toContain('/assets/js/index.js:1:1');
});

test('processWindowErrorEvent falls back to message without stack', () => {
  processWindowErrorEvent({
    error: {message: 'script error'}, type: 'error',
    filename: `${window.location.origin}/assets/js/x.js`, lineno: 5, colno: 10,
  } as ErrorEvent & PromiseRejectionEvent);
  const msgText = document.querySelector('.js-global-error .ui.message')!.textContent;
  expect(msgText).toContain('JavaScript error: script error');
  expect(msgText).toContain('@ 5:10');
});
