/**
 * Form handling utilities
 */

import {addDelegatedEventListener} from '../utils/dom.ts';

/**
 * Prevent duplicate form submission
 * @param form The form element
 * @param submitter The submit button element
 */
function preventDuplicateSubmit(form: HTMLFormElement, submitter: HTMLElement) {
  form.addEventListener('submit', () => {
    submitter.classList.add('disabled');
    submitter.setAttribute('disabled', 'disabled');
  });
}

/**
 * Initialize form submit handlers
 */
export function initFormSubmitHandlers() {
  // Add delegated event listener for forms with data-prevent-duplicate attribute
  addDelegatedEventListener(document, 'submit', 'form[data-prevent-duplicate]', (form: HTMLFormElement) => {
    const submitter = form.querySelector<HTMLElement>('button[type="submit"]');
    if (submitter) {
      preventDuplicateSubmit(form, submitter);
    }
  });
}
