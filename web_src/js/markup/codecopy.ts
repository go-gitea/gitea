import {svg} from '../svg.ts';
import {queryElems} from '../utils/dom.ts';

export function makeCodeCopyButton(attrs: Record<string, string> = {}): HTMLButtonElement {
  const button = document.createElement('button');
  button.classList.add('code-copy', 'ui', 'button');
  button.innerHTML = svg('octicon-copy');
  for (const [key, value] of Object.entries(attrs)) {
    button.setAttribute(key, value);
  }
  return button;
}

export function initMarkupCodeCopy(elMarkup: HTMLElement): void {
  // .markup .code-block code
  queryElems(elMarkup, '.code-block code', (el) => {
    if (!el.textContent) return;
    const btn = makeCodeCopyButton();
    // remove final trailing newline introduced during HTML rendering
    btn.setAttribute('data-clipboard-text', el.textContent.replace(/\r?\n$/, ''));
    // we only want to use `.code-block-container` if it exists, no matter `.code-block` exists or not.
    const btnContainer = el.closest('.code-block-container') ?? el.closest('.code-block');
    btnContainer!.append(btn);
  });
}
