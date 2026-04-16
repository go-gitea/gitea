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

  const errs = document.querySelectorAll('.js-global-error');
  expect(errs.length).toEqual(2);
  expect(errs[0].querySelector('.js-global-error-msg')!.textContent).toBe('test msg 1');
  expect(errs[0].querySelector('.js-global-error-count')!.textContent).toBe(' (2)');
  expect(errs[1].querySelector('.js-global-error-msg')!.textContent).toBe('test msg 2');
  expect(errs[1].querySelector('.js-global-error-count')!.textContent).toBe('');
});

test('showGlobalErrorMessage stores stack for copy', () => {
  showGlobalErrorMessage('hi', 'error', 'at foo (x:1:1)');
  expect(document.querySelector('.js-global-error-stack')!.textContent).toBe('at foo (x:1:1)');
  expect(document.querySelector('.js-global-error-copy')).toBeTruthy();
});

test('processWindowErrorEvent renders stack trace', () => {
  const error = new Error('boom');
  error.stack = `Error: boom\n    at fn (${window.location.origin}/assets/js/index.js:1:1)`;
  processWindowErrorEvent({error, type: 'error'} as ErrorEvent & PromiseRejectionEvent);
  expect(document.querySelector('.js-global-error-msg')!.textContent).toBe('JavaScript error: boom');
  expect(document.querySelector('.js-global-error-stack')!.textContent).toContain('/assets/js/index.js:1:1');
});

test('processWindowErrorEvent falls back to message without stack', () => {
  processWindowErrorEvent({
    error: {message: 'script error'} as Error, type: 'error',
    filename: `${window.location.origin}/assets/js/x.js`, lineno: 5, colno: 10,
  } as ErrorEvent & PromiseRejectionEvent);
  const msgText = document.querySelector('.js-global-error-msg')!.textContent;
  expect(msgText).toContain('JavaScript error: script error');
  expect(msgText).toContain('@ 5:10');
});
