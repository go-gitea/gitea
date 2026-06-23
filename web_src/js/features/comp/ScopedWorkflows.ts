import {addDelegatedEventListener, toggleElem} from '../../utils/dom.ts';

// syncScopedRequiredRow shows a scoped workflow's status-check patterns textarea only while the workflow is required and hides it otherwise.
function syncScopedRequiredRow(checkbox: HTMLInputElement) {
  const row = checkbox.closest('tr')!;
  const textarea = row.querySelector<HTMLTextAreaElement>('.js-scoped-required-patterns')!;
  toggleElem(textarea, checkbox.checked);
  toggleElem(row.querySelector('.js-scoped-required-hint')!, !checkbox.checked); // the "mark as required" hint shown in the textarea's place
  if (checkbox.checked && !textarea.value.trim()) {
    textarea.value = textarea.getAttribute('data-default-pattern')!;
  }
}

export function initScopedWorkflowRequired(form: HTMLElement) {
  for (const checkbox of form.querySelectorAll<HTMLInputElement>('.js-scoped-required-toggle')) {
    syncScopedRequiredRow(checkbox);
  }
  addDelegatedEventListener(form, 'change', '.js-scoped-required-toggle', (checkbox: HTMLInputElement) => syncScopedRequiredRow(checkbox));
}
