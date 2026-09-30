import {addDelegatedEventListener, onInputDebounce, toggleElem} from '../../utils/dom.ts';
import {globMatch} from '../../utils/glob.ts';

// markRowMatchedContexts marks each expected status-check context whose row's textarea patterns match it.
function markRowMatchedContexts(row: HTMLElement) {
  const textarea = row.querySelector<HTMLTextAreaElement>('.js-scoped-required-patterns')!;
  const patterns = textarea.value.split(/[\r\n]+/).map((p) => p.trim()).filter(Boolean);
  for (const ctxEl of row.querySelectorAll<HTMLElement>('.js-scoped-context')) {
    const context = ctxEl.getAttribute('data-context')!;
    const matched = patterns.some((p) => globMatch(context, p));
    toggleElem(ctxEl.parentElement!.querySelector('.js-scoped-context-matched')!, matched);
  }
}

// syncScopedRequiredRow shows a scoped workflow's status-check patterns textarea (and its expected-checks preview) only while the workflow is required.
function syncScopedRequiredRow(checkbox: HTMLInputElement) {
  const row = checkbox.closest('tr')!;
  const textarea = row.querySelector<HTMLTextAreaElement>('.js-scoped-required-patterns')!;
  toggleElem(textarea, checkbox.checked);
  toggleElem(row.querySelector('.js-scoped-required-hint')!, !checkbox.checked); // the "mark as required" hint shown in the textarea's place
  const contexts = row.querySelector('.js-scoped-required-contexts'); // only rendered when the workflow has expected checks
  if (contexts) toggleElem(contexts, checkbox.checked);
  if (checkbox.checked && !textarea.value.trim()) {
    textarea.value = textarea.getAttribute('data-default-pattern')!;
  }
  if (checkbox.checked) markRowMatchedContexts(row);
}

export function initScopedWorkflowRequired(form: HTMLElement) {
  for (const checkbox of form.querySelectorAll<HTMLInputElement>('.js-scoped-required-toggle')) {
    syncScopedRequiredRow(checkbox);
  }
  for (const textarea of form.querySelectorAll<HTMLTextAreaElement>('.js-scoped-required-patterns')) {
    const row = textarea.closest('tr')!;
    textarea.addEventListener('input', onInputDebounce(() => markRowMatchedContexts(row)));
  }
  addDelegatedEventListener(form, 'change', '.js-scoped-required-toggle', (checkbox: HTMLInputElement) => syncScopedRequiredRow(checkbox));
}
