import '../webcomponents/diff-commit-selector.ts';

export function initDiffCommitSelect() {
  const el = document.querySelector('#diff-commit-select');
  if (!el) return;

  // The web component self-initializes when mounted
  // Just ensure the element exists and has the required attributes
  el.classList.remove('is-loading');
}
