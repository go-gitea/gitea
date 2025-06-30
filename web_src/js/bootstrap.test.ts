import {showGlobalErrorMessage} from './bootstrap.ts';
import {html} from './utils/html.ts';

test('showGlobalErrorMessage', () => {
  document.body.innerHTML = html`<div class="page-content"></div>`;
  showGlobalErrorMessage('test msg 1');
  showGlobalErrorMessage('test msg 2');
  showGlobalErrorMessage('test msg 1'); // duplicated

  expect(document.body.innerHTML).toContain('>test msg 1 (2)<');
  expect(document.body.innerHTML).toContain('>test msg 2<');
  expect(document.querySelectorAll('.js-global-error').length).toEqual(2);
});
