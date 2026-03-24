import {linkLabelAndInput} from './base.ts';

export function initAriaCheckboxPatch() {
  // link the label and the input element so it's clickable and accessible
  for (const el of document.querySelectorAll('.ui.checkbox')) {
    if (el.hasAttribute('data-checkbox-patched')) continue;
    const label = el.querySelector('label');
    const input = el.querySelector('input');
    if (!label || !input) continue;
    linkLabelAndInput(label, input);
    // prevent accidental text selection on double-click
    label.addEventListener('mousedown', (e: MouseEvent) => {
      if (e.detail === 2) e.preventDefault();
    });
    el.setAttribute('data-checkbox-patched', 'true');
  }
}
