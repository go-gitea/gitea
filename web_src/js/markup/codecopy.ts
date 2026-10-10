import {svg} from '../svg.ts';
import {createElementFromAttrs, generateElemId, queryElems} from '../utils/dom.ts';

export function makeCodeCopyButton(attrs: Record<string, string> = {}): HTMLButtonElement {
  const btn = createElementFromAttrs<HTMLButtonElement>('button', {
    class: 'ui compact icon button code-copy auto-hide-control',
    type: 'button',
    ...attrs,
  });
  btn.innerHTML = svg('octicon-copy');
  return btn;
}

export function initMarkupCodeCopy(elMarkup: HTMLElement): void {
  queryElems(elMarkup, '.code-block code', (el) => {
    // the code block's content can change (e.g.: debian package page: select different distributions),
    // so use "data-clipboard-target" to copy the target's textContent, also always respect the content's end of block newline.
    if (!el.getAttribute('id')) el.setAttribute('id', generateElemId('_code_block_'));
    const btn = makeCodeCopyButton({'data-clipboard-target': `#${el.getAttribute('id')}`});
    // we only want to use `.code-block-container` if it exists, no matter `.code-block` exists or not.
    const btnContainer = el.closest('.code-block-container') ?? el.closest('.code-block')!;
    btnContainer.append(btn);
  });
}
