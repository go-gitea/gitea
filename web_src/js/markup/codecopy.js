import {svg} from '../svg.js';

export function renderCodeCopy() {
  const els = document.querySelectorAll('.markup .code-block code');
  if (!els.length) return;

  const button = document.createElement('button');
  button.classList.add('code-copy', 'ui', 'button');
  button.innerHTML = svg('octicon-copy');

  for (const el of els) {
    const btn = button.cloneNode(true);
    const code = (el.textContent || '').replace(/\r?\n$/, '');
    btn.setAttribute('data-clipboard-text', code);
    el.after(btn);
  }
}
