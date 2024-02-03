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

  const MAX_CHARS = 1000;
  const MAX_SIZE = 25;
  const MAX_EXPAND = 1000;

  for (const el of els) {
    const target = targetElement(el);
    if (target.hasAttribute('data-render-done')) continue;
    const source = el.textContent;

    if (source.length > MAX_CHARS) {
      displayError(target, new Error(`Math source of ${source.length} characters exceeds the maximum allowed length of ${MAX_CHARS}.`));
      continue;
    }

    const displayMode = el.classList.contains('display');
    const nodeName = displayMode ? 'p' : 'span';

    try {
      const tempEl = document.createElement(nodeName);
      katex.render(source, tempEl, {
        maxSize: MAX_SIZE,
        maxExpand: MAX_EXPAND,
        displayMode,
      });
      target.replaceWith(tempEl);
    } catch (error) {
      displayError(target, error);
    }
  }
}
