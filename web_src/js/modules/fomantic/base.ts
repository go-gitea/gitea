import {generateElemId, queryElems} from '../../utils/dom.ts';

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

function patchLabels(parent: ParentNode, containerSelector: string, labelSelector: string, inputSelector: string, marker: string) {
  // Sample layout for this function:
  // <div parent>
  //   <div container><label/><input/></div>
  //   <div container><label/><input/></div>
  // </div>
  //
  // OR the parent is also the container:
  // <div parent container><label/><input/></div>

  const patchLabelContainer = (container: Element) => {
    if (container.hasAttribute(marker)) return;
    const label = container.querySelector(labelSelector);
    const input = container.querySelector(inputSelector);
    if (!label || !input) return;
    linkLabelAndInput(label, input);
    container.setAttribute(marker, 'true');
  };
  queryElems(parent, containerSelector, patchLabelContainer);
  if (parent instanceof Element && parent.matches(containerSelector)) patchLabelContainer(parent);
}

// link labels and inputs in `.ui.checkbox` and `.ui.form .field` so labels are clickable and accessible
export function initAriaLabels(container: ParentNode) {
  patchLabels(container, '.ui.checkbox', 'label', 'input', 'data-checkbox-patched');
  patchLabels(container, '.ui.form .field', ':scope > label', ':scope > input, :scope > select', 'data-field-patched');
}

export function fomanticQuery(s: string | Element | NodeListOf<Element>): ReturnType<typeof $> {
  // intentionally make it only work for query selector, it isn't used for creating HTML elements (for safety)
  return typeof s === 'string' ? $(document).find(s) : $(s);
}
