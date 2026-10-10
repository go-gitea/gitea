import {registerGlobalInitFunc} from '../modules/observer.ts';
import {queryElems} from '../utils/dom.ts';

export function initPackagesManualVars(el: HTMLElement) {
  queryElems(el, 'span', (elSpan: HTMLElement) => {
    const text = elSpan.textContent;
    if (!text.startsWith('$')) return;
    elSpan.setAttribute('data-code-var', text.substring(1));
  });
  const syncVar = (elSelect: HTMLSelectElement) => {
    const varName = elSelect.getAttribute('data-code-var')!;
    const elSpans = el.querySelectorAll(`span[data-code-var="${CSS.escape(varName)}"]`);
    if (elSpans.length === 0) throw new Error(`No span found for variable ${varName}`);
    for (const elSpan of elSpans) elSpan.textContent = elSelect.value;
  };
  queryElems(el, 'select[data-code-var]', (elSelect: HTMLSelectElement) => {
    syncVar(elSelect);
    elSelect.addEventListener('change', () => syncVar(elSelect));
  });
}

export function initPackagesView() {
  registerGlobalInitFunc('initPackagesManualVars', initPackagesManualVars);
}
