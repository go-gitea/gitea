import {isGiteaError, processWindowErrorEvent, showGlobalError} from './errors.ts';

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

test('showGlobalError', () => {
  document.body.innerHTML = '<div class="page-content"></div>';
  showGlobalError(new Error('test msg 1'), {noStack: true});
  showGlobalError(new Error('test msg 2'), {noStack: true});
  showGlobalError(new Error('test msg 1'), {noStack: true}); // duplicated

  const errs = document.querySelectorAll('.js-global-error');
  expect(errs.length).toEqual(2);
  expect(errs[0].querySelector('.js-global-error-msg')!.textContent).toBe('JavaScript error: test msg 1');
  expect(errs[0].querySelector('.js-global-error-count')!.textContent).toBe(' (2)');
  expect(errs[1].querySelector('.js-global-error-msg')!.textContent).toBe('JavaScript error: test msg 2');
  expect(errs[1].querySelector('.js-global-error-count')!.textContent).toBe('');
});

test('showGlobalError stores stack hidden for copy', () => {
  document.body.innerHTML = '<div class="page-content"></div>';
  const err = new Error('hi');
  err.stack = 'at foo (x:1:1)\nat bar (y:2:2)';
  showGlobalError(err);
  const stackEl = document.querySelector<HTMLElement>('.js-global-error-stack')!;
  expect(stackEl.tagName).toBe('PRE');
  expect(stackEl.classList.contains('tw-hidden')).toBe(true);
  expect(stackEl.textContent).toBe('at foo (x:1:1)\nat bar (y:2:2)');
  expect(document.querySelector('.js-global-error-copy')).toBeTruthy();
});

test('showGlobalError noStack hides stack', () => {
  document.body.innerHTML = '<div class="page-content"></div>';
  const err = new Error('warning');
  err.stack = 'stack content';
  showGlobalError(err, {msgType: 'warning', noStack: true});
  expect(document.querySelector('.js-global-error-msg')!.textContent).toBe('warning');
  expect(document.querySelector('.js-global-error-stack')!.textContent).toBe('');
});

test('processWindowErrorEvent renders stack trace', () => {
  document.body.innerHTML = '<div class="page-content"></div>';
  const error = new Error('boom');
  error.stack = `Error: boom\n    at fn (${window.location.origin}/assets/js/index.js:1:1)`;
  processWindowErrorEvent({error, type: 'error'} as ErrorEvent & PromiseRejectionEvent);
  expect(document.querySelector('.js-global-error-msg')!.textContent).toBe('JavaScript error: boom');
  expect(document.querySelector('.js-global-error-stack')!.textContent).toContain('/assets/js/index.js:1:1');
});

test('processWindowErrorEvent falls back to message without stack', () => {
  document.body.innerHTML = '<div class="page-content"></div>';
  const stacklessError = {message: 'script error'} as Error;
  processWindowErrorEvent({
    error: stacklessError, type: 'error',
    filename: `${window.location.origin}/assets/js/x.js`, lineno: 5, colno: 10,
  } as ErrorEvent & PromiseRejectionEvent);
  const msgText = document.querySelector('.js-global-error-msg')!.textContent;
  expect(msgText).toContain('JavaScript error: script error');
  expect(msgText).toContain('@ 5:10');
  expect(document.querySelector('.js-global-error-stack')!.textContent).toBe('');
});
