export function handleGlobalEnterQuickSubmit(target) {
  let form = target.closest('form');
  if (form) {
    if (!form.checkValidity()) {
      form.reportValidity();
    } else {
      // here use the event to trigger the submit event (instead of calling `submit()` method directly)
      // otherwise the `areYouSure` handler won't be executed, then there will be an annoying "confirm to leave" dialog
      form.dispatchEvent(new SubmitEvent('submit', {bubbles: true, cancelable: true}));
    }
    return true;
  }
  form = target.closest('.ui.form');
  if (form) {
    form.querySelector('.ui.primary.button')?.click();
    return true;
  }
  return false;
}
