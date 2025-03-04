import {displayError} from './common.ts';

function targetElement(el: Element): {target: Element, displayAsBlock: boolean} {
  // The target element is either the parent "code block with loading indicator", or itself
  // It is designed to work for 2 cases (guaranteed by backend code):
  // * <pre class="code-block is-loading"><code class="language-math display">...</code></pre>
  // * <code class="language-math">...</code>
  return {
    target: el.closest('.code-block.is-loading') ?? el,
    displayAsBlock: el.classList.contains('display'),
  };
}

export async function initMarkupCodeMath(elMarkup: HTMLElement): Promise<void> {
  const el = elMarkup.querySelector('code.language-math'); // .markup code.language-math'
  if (!el) return;

  const [{default: katex}] = await Promise.all([
    import(/* webpackChunkName: "katex" */'katex'),
    import(/* webpackChunkName: "katex" */'katex/dist/katex.css'),
  ]);

  const MAX_CHARS = 1000;
  const MAX_SIZE = 25;
  const MAX_EXPAND = 1000;

  const {target, displayAsBlock} = targetElement(el);
  if (target.hasAttribute('data-render-done')) return;
  const source = el.textContent;

  if (source.length > MAX_CHARS) {
    displayError(target, new Error(`Math source of ${source.length} characters exceeds the maximum allowed length of ${MAX_CHARS}.`));
    return;
  }
  try {
    const tempEl = document.createElement(displayAsBlock ? 'p' : 'span');
    katex.render(source, tempEl, {
      maxSize: MAX_SIZE,
      maxExpand: MAX_EXPAND,
      displayMode: displayAsBlock, // katex: true for display (block) mode, false for inline mode
    });
    target.replaceWith(tempEl);
  } catch (error) {
    displayError(target, error);
  }
}
