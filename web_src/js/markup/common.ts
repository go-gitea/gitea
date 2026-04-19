export function displayError(el: Element, err: unknown): void {
  const e = err as Error;
  el.classList.remove('is-loading');
  const errorNode = document.createElement('pre');
  errorNode.setAttribute('class', 'ui message error markup-block-error');
  errorNode.textContent = e.message || String(e);
  el.before(errorNode);
  el.setAttribute('data-render-done', 'true');
}
