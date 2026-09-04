type FormField = HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;
type AreYouSureState = {onDirtyChange?: (dirty: boolean) => void, dirty: boolean};

const areYouSureStates = new WeakMap<HTMLFormElement, AreYouSureState>();
const originalValues = new WeakMap<FormField, string>();

function fieldValue(field: FormField): string {
  if (field instanceof HTMLSelectElement) {
    return JSON.stringify(Array.from(field.selectedOptions, (option) => option.value));
  }
  if (field instanceof HTMLInputElement && ['checkbox', 'radio'].includes(field.type)) return String(field.checked);
  return field.value;
}

function trackedFields(form: HTMLFormElement): FormField[] {
  return Array.from(form.querySelectorAll<FormField>('input:not([type=submit], [type=button]), select, textarea')).filter((field) => {
    return field.name && !field.closest('.ays-ignore');
  });
}

function setDirty(form: HTMLFormElement, dirty: boolean) {
  const state = areYouSureStates.get(form)!;
  if (state.dirty === dirty) return;
  state.dirty = dirty;
  state.onDirtyChange?.(dirty);
}

export function applyAreYouSure(form: HTMLFormElement, onDirtyChange?: (dirty: boolean) => void) {
  if (!areYouSureStates.has(form)) {
    const isFieldDirty = (field: FormField) => originalValues.has(field) && originalValues.get(field) !== fieldValue(field);
    const checkDirty = () => setDirty(form, trackedFields(form).some(isFieldDirty));
    // keyup: some keydown handlers set values without an input event
    for (const eventType of ['input', 'change', 'keyup']) form.addEventListener(eventType, checkDirty);
    for (const eventType of ['submit', 'reset']) form.addEventListener(eventType, () => setDirty(form, false));
  }
  areYouSureStates.set(form, {onDirtyChange, dirty: false});
  for (const field of trackedFields(form)) originalValues.set(field, fieldValue(field));
}

export function ignoreAreYouSure(el: Element) {
  el.classList.add('ignore-dirty');
}

export function shouldTriggerAreYouSure(): boolean {
  return Array.from(document.querySelectorAll<HTMLFormElement>('form:not(.ignore-dirty)')).some((form) => {
    return areYouSureStates.get(form)?.dirty && !form.closest('.tw-hidden');
  });
}

export function initGlobalFormDirtyLeaveConfirm() {
  if (document.querySelector('.page-content.user.signin')) return;
  for (const form of document.querySelectorAll<HTMLFormElement>('form:not(.ignore-dirty)')) applyAreYouSure(form);
  window.addEventListener('beforeunload', (e) => {
    if (shouldTriggerAreYouSure()) e.preventDefault();
  });
}
