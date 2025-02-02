import {svg} from '../svg.ts';

export function makeCodeCopyButton(): HTMLButtonElement {
  const button = document.createElement('button');
  button.classList.add('code-copy', 'ui', 'button');
  button.innerHTML = svg('octicon-copy');
  return button;
}

export function renderCodeCopy(): void {
  const els = document.querySelectorAll('.markup .code-block code');
  if (!els.length) return;

  for (const el of els) {
    if (!el.textContent) continue;
    const btn = makeCodeCopyButton();
    // remove final trailing newline introduced during HTML rendering
    btn.setAttribute('data-clipboard-text', el.textContent.replace(/\r?\n$/, ''));
    el.after(btn);
  }
}
