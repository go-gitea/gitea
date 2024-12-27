export function initAutoFocusEnd() {
  for (const el of document.querySelectorAll<HTMLInputElement>('.js-autofocus-end')) {
    el.focus(); // expects only one such element on one page. If there are many, then the last one gets the focus.
    el.setSelectionRange(el.value.length, el.value.length);
  }
}
