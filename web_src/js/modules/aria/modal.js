import $ from 'jquery';

const fomanticModalFn = $.fn.modal;

// use our own `$.fn.modal` to patch Fomantic's modal module
export function initAriaModalPatch() {
  if ($.fn.modal === ariaModalFn) throw new Error('initAriaModalPatch could only be called once');
  $.fn.modal = ariaModalFn;
  ariaModalFn.settings = fomanticModalFn.settings;
}

// the patched `$.fn.modal` modal function
// * it does the one-time attaching on the first call
function ariaModalFn(...args) {
  const ret = fomanticModalFn.apply(this, args);
  if (args[0] === 'show' || args[0]?.autoShow) {
    for (const el of this) {
      // If there is a form in the modal, there might be a "cancel" button before "ok" button (all buttons are "type=submit" by default).
      // In such case, the "Enter" key will trigger the "cancel" button instead of "ok" button, then the dialog will be closed.
      // It breaks the user experience - the "Enter" key should confirm the dialog and submit the form.
      // So, all "cancel" buttons without "[type]" must be marked as "type=button".
      $(el).find('form button.cancel:not([type])').attr('type', 'button');
    }
  }
  return ret;
}
