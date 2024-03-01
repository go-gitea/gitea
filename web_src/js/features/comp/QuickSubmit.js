export function handleGlobalEnterQuickSubmit(target) {
  const form = target.closest('form');
  if (form) {
    if (!form.checkValidity()) {
      form.reportValidity();
      return;
    }

    // here use the event to trigger the submit event (instead of calling `submit()` method directly)
    // otherwise the `areYouSure` handler won't be executed, then there will be an annoying "confirm to leave" dialog
    form.dispatchEvent(new SubmitEvent('submit', {bubbles: true, cancelable: true}));
  } else {
    // if no form, then the editor is for an AJAX request, dispatch an event to the target, let the target's event handler to do the AJAX request.
    // the 'ce-' prefix means this is a CustomEvent
    target.dispatchEvent(new CustomEvent('ce-quick-submit', {bubbles: true}));
  }
}
