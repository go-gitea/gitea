import {generateElemId, queryElems} from '../utils/dom.ts';

function patchLabels(parent: ParentNode, containerSelector: string, labelSelector: string, inputSelector: string, marker: string) {
  const patchLabelContainer = (container: Element) => {
    if (container.hasAttribute(marker)) return;
    const label = container.querySelector(labelSelector);
    const input = container.querySelector(inputSelector);
    if (!label || !input) return;
    if (!label.getAttribute('for')) label.setAttribute('for', input.id ||= generateElemId('_aria_label_input_'));
    container.setAttribute(marker, 'true');
  };
  queryElems(parent, containerSelector, patchLabelContainer);
  if (parent instanceof Element && parent.matches(containerSelector)) patchLabelContainer(parent);
}

// link labels and inputs in `.ui.checkbox` and `.ui.form .field` so labels are clickable and accessible
export function initAriaLabels(container: ParentNode) {
  patchLabels(container, '.ui.checkbox', 'label', 'input', 'data-checkbox-patched');
  patchLabels(container, '.ui.form .field', ':scope > label', ':scope > input, :scope > select, :scope > .select-field > select, :scope > textarea', 'data-field-patched');
}
