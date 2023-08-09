import {svg} from '../svg.js';

export function makeCodeCopyButton() {
  const button = document.createElement('button');
  button.classList.add('code-copy', 'ui', 'button');
  button.innerHTML = svg('octicon-copy');
  return button;
}

export function renderCodeCopy() {
  const els = document.querySelectorAll('.markup .code-block code');
  if (!els.length) return;

  for (const el of els) {
    const btn = makeCodeCopyButton();
    btn.setAttribute('data-clipboard-text', el.textContent);
    el.after(btn);
  }
}
