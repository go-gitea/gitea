export function displayError(el, err) {
  el.classList.remove('is-loading');
  const errorNode = document.createElement('div');
  errorNode.setAttribute('class', 'ui message error markup-block-error gt-mono');
  errorNode.textContent = err.str || err.message || String(err);
  el.before(errorNode);
  el.setAttribute('data-done', 'true');
}
