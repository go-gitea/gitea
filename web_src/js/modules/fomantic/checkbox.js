import {generateAriaId} from './base.js';

export function initAriaCheckboxPatch() {
  // link the label and the input element so it's clickable and accessible
  for (const el of document.querySelectorAll('.ui.checkbox')) {
    if (el.hasAttribute('data-checkbox-patched')) continue;
    const label = el.querySelector('label');
    const input = el.querySelector('input');
    if (!label || !input || input.getAttribute('id') || label.getAttribute('for')) continue;
    const id = generateAriaId();
    input.setAttribute('id', id);
    label.setAttribute('for', id);
    el.setAttribute('data-checkbox-patched', 'true');
  }
}
