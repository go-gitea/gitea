function displayError(el, err) {
  const target = targetElement(el);
  target.classList.remove('is-loading');
  const errorNode = document.createElement('div');
  errorNode.setAttribute('class', 'ui message error markup-block-error gt-mono');
  errorNode.textContent = err.str || err.message || String(err);
  target.before(errorNode);
}

function targetElement(el) {
  // The target element is either the current element if it has the `is-loading` class or the pre that contains it
  return el.classList.contains('is-loading') ? el : el.closest('pre');
}

export async function renderMath() {
  const els = document.querySelectorAll('.markup code.language-math');
  if (!els.length) return;

  const [{default: katex}] = await Promise.all([
    import(/* webpackChunkName: "katex" */'katex'),
    import(/* webpackChunkName: "katex" */'katex/dist/katex.css'),
  ]);

  for (const el of els) {
    const source = el.textContent;
    const displayMode = el.classList.contains('display');
    const nodeName = displayMode ? 'p' : 'span';

    try {
      const tempEl = document.createElement(nodeName);
      katex.render(source, tempEl, {
        maxSize: 25,
        maxExpand: 50,
        displayMode,
      });
      targetElement(el).replaceWith(tempEl);
    } catch (error) {
      displayError(el, error);
    }
  }
}
