import {generateElemId} from '../utils/dom.ts';

function linkLabelAndInput(label: Element, input: Element) {
  const labelFor = label.getAttribute('for');
  const inputId = input.getAttribute('id');

  if (inputId && !labelFor) { // missing "for"
    label.setAttribute('for', inputId);
  } else if (!inputId && !labelFor) { // missing both "id" and "for"
    const id = generateElemId('_aria_label_input_');
    input.setAttribute('id', id);
    label.setAttribute('for', id);
  }
}

function patchLabels(containerSelector: string, labelSelector: string, inputSelector: string, marker: string) {
  for (const el of document.querySelectorAll(containerSelector)) {
    if (el.hasAttribute(marker)) continue;
    const label = el.querySelector(labelSelector);
    const input = el.querySelector(inputSelector);
    if (!label || !input) continue;
    linkLabelAndInput(label, input);
    el.setAttribute(marker, 'true');
  }
}

// link labels and inputs in `.ui.checkbox` and `.ui.form .field` so labels are clickable and accessible
export function initAriaLabels() {
  patchLabels('.ui.checkbox', 'label', 'input', 'data-checkbox-patched');
  patchLabels('.ui.form .field', ':scope > label', ':scope > input, :scope > select', 'data-field-patched');
}
