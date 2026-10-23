import {svg} from '../svg.ts';
import {createElementFromAttrs, queryElems} from '../utils/dom.ts';
import {copyToClipboardWithFeedback} from '../modules/clipboard.ts';

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
    // so read it on click, which also survives math and mermaid replacing the block
    const btn = makeCodeCopyButton();
    btn.addEventListener('click', () => copyToClipboardWithFeedback(btn, el.textContent.replace(/\r?\n$/, '')));
    // we only want to use `.code-block-container` if it exists, no matter `.code-block` exists or not.
    const btnContainer = el.closest('.code-block-container') ?? el.closest('.code-block')!;
    btnContainer.append(btn);
  });
}
