export function displayError(el, err) {
  el.classList.remove('is-loading');
  const errorNode = document.createElement('pre');
  errorNode.setAttribute('class', 'ui message error markup-block-error');
  errorNode.textContent = err.str || err.message || String(err);
  el.before(errorNode);
  el.setAttribute('data-render-done', 'true');
}
