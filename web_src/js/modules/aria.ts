import {generateElemId, queryElems} from '../utils/dom.ts';

function patchLabels(parent: ParentNode, containerSelector: string, labelSelector: string, inputSelector: string) {
  const patchLabelContainer = (container: Element) => {
    const label = container.querySelector(labelSelector);
    if (!label || label.getAttribute('for')) return;
    const input = container.querySelector(inputSelector);
    if (input) label.setAttribute('for', input.id ||= generateElemId('_aria_label_input_'));
  };
  queryElems(parent, containerSelector, patchLabelContainer);
  if (parent instanceof Element && parent.matches(containerSelector)) patchLabelContainer(parent);
}

export function initAriaLabels(container: ParentNode) {
  patchLabels(container, '.ui.checkbox', 'label', 'input');
  patchLabels(container, '.ui.form .field', ':scope > label', ':scope > input, :scope > select, :scope > .select-field > select, :scope > textarea');
}
