import {svg} from '../svg.ts';

export function makeCodeCopyButton(): HTMLButtonElement {
  const button = document.createElement('button');
  button.classList.add('code-copy', 'ui', 'button');
  button.innerHTML = svg('octicon-copy');
  return button;
}

export function initMarkupCodeCopy(elMarkup: HTMLElement): void {
  const els = elMarkup.querySelectorAll('.code-wrapper'); // .markup .code-block code

  for (const el of els) {
    if (!el || !el.textContent) return;
    const btn = makeCodeCopyButton();
    // remove final trailing newline introduced during HTML rendering
    btn.setAttribute('data-clipboard-text', el.textContent.replace(/\r?\n$/, ''));
    el.append(btn);
  }
}
