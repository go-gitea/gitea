import {svgNode} from '../svg.js';
const {copied, copy_link_error} = window.i18n;

export function renderCodeCopy() {
  const els = document.querySelectorAll('.markup .code-block code');
  if (!els?.length) return;

  const button = document.createElement('button');
  button.classList.add('code-copy', 'ui', 'button');
  button.setAttribute('data-success', copied);
  button.setAttribute('data-error', copy_link_error);
  button.setAttribute('data-variation', 'inverted tiny');
  button.appendChild(svgNode('octicon-copy'));

  for (const el of els) {
    const btn = button.cloneNode(true);
    btn.setAttribute('data-clipboard-text', el.textContent);
    el.after(btn);
  }
}
