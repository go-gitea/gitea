import {isGiteaError, showGlobalErrorMessage} from './errors.ts';

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
  document.body.innerHTML = '<div class="page-content"></div>';
  showGlobalErrorMessage('test msg 1');
  showGlobalErrorMessage('test msg 2');
  showGlobalErrorMessage('test msg 1'); // duplicated

  expect(document.body.innerHTML).toContain('>test msg 1 (2)<');
  expect(document.body.innerHTML).toContain('>test msg 2<');
  expect(document.querySelectorAll('.js-global-error').length).toEqual(2);
});
