import {svg} from '../svg.ts';
import {createElementFromAttrs, queryElems} from '../utils/dom.ts';

export function makeCodeCopyButton(attrs: Record<string, string> = {}): HTMLButtonElement {
  const btn = createElementFromAttrs<HTMLButtonElement>('button', {
    class: 'ui compact icon button code-copy auto-hide-control',
    ...attrs,
  });
  btn.innerHTML = svg('octicon-copy');
  return btn;
}

export function initMarkupCodeCopy(elMarkup: HTMLElement): void {
  // .markup .code-block code
  queryElems(elMarkup, '.code-block code', (el) => {
    if (!el.textContent) return;
    // remove final trailing newline introduced during HTML rendering
    const btn = makeCodeCopyButton({
      'data-clipboard-text': el.textContent.replace(/\r?\n$/, ''),
    });
    // we only want to use `.code-block-container` if it exists, no matter `.code-block` exists or not.
    const btnContainer = el.closest('.code-block-container') ?? el.closest('.code-block')!;
    btnContainer.append(btn);
  });
}
