import {initPackagesManualVars} from './packages.ts';
import {createElementFromHTML} from '../utils/dom.ts';

test('initPackagesManualVars', () => {
  const el = createElementFromHTML<HTMLElement>(`
<div>
  <select data-code-var="foo"><option value="v1"></option><option value="v2"></option></select>
  <span>$foo</span>
</div>
`);
  initPackagesManualVars(el);
  const elSelect = el.querySelector<HTMLSelectElement>('select')!;
  const elSpan = el.querySelector('span')!;
  expect(elSpan.textContent).toBe('v1');
  elSelect.value = 'v2';
  elSelect.dispatchEvent(new Event('change'));
  expect(elSpan.textContent).toBe('v2');
});
