import {displayError} from './common.js';

function targetElement(el) {
  // The target element is either the current element if it has the
  // `is-loading` class or the pre that contains it
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
    const target = targetElement(el);
    if (target.hasAttribute('data-render-done')) continue;
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
      target.replaceWith(tempEl);
    } catch (error) {
      displayError(target, error);
    }
  }
}
