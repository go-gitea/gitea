import {displayError} from './common.ts';
import {queryElems} from '../utils/dom.ts';

function targetElement(elCode: Element): {target: Element, displayAsBlock: boolean} {
  // The target element is either the parent "code block with loading indicator", or itself
  // It is designed to work for 2 cases (guaranteed by backend code):
  // * <pre class="code-block"><code class="language-math">...</code></pre>
  // * <code class="language-math">...</code>
  const elPre = elCode.parentElement?.matches('pre.code-block') ? elCode.parentElement : null;
  return {
    target: elPre ?? elCode,
    displayAsBlock: elPre !== null,
  };
}

export async function initMarkupCodeMath(elMarkup: HTMLElement): Promise<void> {
  // .markup code.language-math'
  queryElems(elMarkup, 'code.language-math', async (el) => {
    const [{default: katex}] = await Promise.all([
      import('katex'),
      import('katex/dist/katex.css'),
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
  });
}
