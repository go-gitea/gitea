import {generateAriaId} from './base.js';

export function initAriaCheckboxPatch() {
  // link the label and the input element so it's clickable and accessible
  for (const el of document.querySelectorAll('.ui.checkbox')) {
    if (el.hasAttribute('data-checkbox-patched')) continue;
    const label = el.querySelector('label');
    const input = el.querySelector('input');
    if (!label || !input) continue;
    const inputId = input.getAttribute('id');
    const labelFor = label.getAttribute('for');

    if (inputId && !labelFor) { // missing "for"
      label.setAttribute('for', inputId);
    } else if (!inputId && !labelFor) { // missing both "id" and "for"
      const id = generateAriaId();
      input.setAttribute('id', id);
      label.setAttribute('for', id);
    } else {
      continue;
    }
    el.setAttribute('data-checkbox-patched', 'true');
  }
}
