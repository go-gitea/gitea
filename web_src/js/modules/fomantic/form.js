import {linkLabelAndInput} from './base.js';

export function initAriaFormFieldPatch() {
  // link the label and the input element so it's clickable and accessible
  for (const el of document.querySelectorAll('.ui.form .field')) {
    if (el.hasAttribute('data-field-patched')) continue;
    const label = el.querySelector(':scope > label');
    const input = el.querySelector(':scope > input');
    if (!label || !input) continue;
    linkLabelAndInput(label, input);
    el.setAttribute('data-field-patched', 'true');
  }
}
