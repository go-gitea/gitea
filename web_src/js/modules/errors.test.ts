import {isCodeNestError, processWindowErrorEvent, showGlobalErrorMessage} from './errors.ts';

beforeEach(() => {
  document.body.innerHTML = '<div class="page-content"></div>';
});

test('isCodeNestError', () => {
  const {origin} = window.location;
  expect(isCodeNestError('', '')).toBe(true);
  expect(isCodeNestError('moz-extension://abc/content.js', '')).toBe(false);
  expect(isCodeNestError('safari-extension://abc/content.js', '')).toBe(false);
  expect(isCodeNestError('safari-web-extension://abc/content.js', '')).toBe(false);
  expect(isCodeNestError('chrome-extension://abc/content.js', '')).toBe(false);
  expect(isCodeNestError('https://other-site.com/script.js', '')).toBe(false);
  expect(isCodeNestError(`${origin}/some/page`, '')).toBe(true);
  expect(isCodeNestError(`${origin}/assets/js/index.abc123.js`, '')).toBe(true);
  expect(isCodeNestError('', `Error\n    at chrome-extension://abc/content.js:1:1`)).toBe(false);
  expect(isCodeNestError('', `Error\n    at https://other-site.com/script.js:1:1`)).toBe(false);
  expect(isCodeNestError('', `Error\n    at ${origin}/assets/js/index.abc123.js:1:1`)).toBe(true);
  expect(isCodeNestError('', `Error\n    at ${origin}/web_src/js/index.ts:1:1`)).toBe(false);
  expect(isCodeNestError(`${origin}/assets/js/index.js`, `Error\n    at chrome-extension://abc/content.js:1:1`)).toBe(false);
  vi.spyOn(window.config, 'runModeIsProd', 'get').mockReturnValue(false);
  expect(isCodeNestError('', `Error\n    at ${origin}/web_src/js/index.ts:1:1`)).toBe(true);
  vi.restoreAllMocks();
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
