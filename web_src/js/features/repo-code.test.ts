import {selectRange} from './repo-code.ts';

test('selectRange', () => {
  document.body.innerHTML = `<table class="code-view"><tr><td class="lines-num"><span data-line-number="1"></span></td></tr></table>`;
  expect(selectRange('L0')).toBeNull();
  expect(selectRange('L1')).toBe(document.querySelector('[data-line-number="1"]'));
});
