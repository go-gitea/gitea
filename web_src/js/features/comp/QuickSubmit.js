import $ from 'jquery';

export function handleGlobalEnterQuickSubmit(target) {
  const $target = $(target);
  const $form = $(target).closest('form');
  if ($form.length) {
    // here use the event to trigger the submit event (instead of calling `submit()` method directly)
    // otherwise the `areYouSure` handler won't be executed, then there will be an annoying "confirm to leave" dialog
    if ($form[0].checkValidity()) {
      $form.trigger('submit');
    }
  } else {
    // if no form, then the editor is for an AJAX request, dispatch an event to the target, let the target's event handler to do the AJAX request.
    // the 'ce-' prefix means this is a CustomEvent
    $target.trigger('ce-quick-submit');
  }
}
