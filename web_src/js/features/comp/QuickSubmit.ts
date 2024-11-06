import {querySingleVisibleElem} from '../../utils/dom.ts';

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
    // A form should only have at most one "primary" button to do quick-submit.
    // Here we don't use a special class to mark the primary button,
    // because there could be a lot of forms with a primary button, the quick submit should work out-of-box,
    // but not keeps asking developers to add that special class again and again (it could be forgotten easily)
    querySingleVisibleElem<HTMLButtonElement>(form, '.ui.primary.button')?.click();
    return true;
  }
  return false;
}
