import $ from 'jquery';
import {generateElemId} from '../../utils/dom.ts';

export function linkLabelAndInput(label: Element, input: Element) {
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

export function fomanticQuery(s: string | Element | NodeListOf<Element>): ReturnType<typeof $> {
  // intentionally make it only work for query selector, it isn't used for creating HTML elements (for safety)
  return typeof s === 'string' ? $(document).find(s) : $(s);
}
