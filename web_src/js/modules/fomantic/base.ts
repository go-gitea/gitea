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

export function fomanticQuery(s: string) {
  return $(document).find(s);
}
