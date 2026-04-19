import {errorMessage} from '../utils/error.ts';

export function displayError(el: Element, err: unknown): void {
  el.classList.remove('is-loading');
  const errorNode = document.createElement('pre');
  errorNode.setAttribute('class', 'ui message error markup-block-error');
  errorNode.textContent = errorMessage(err) || String(err);
  el.before(errorNode);
  el.setAttribute('data-render-done', 'true');
}
