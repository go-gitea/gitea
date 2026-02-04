import $ from 'jquery';
import type {FomanticInitFunction} from '../../types.ts';
import {queryElems} from '../../utils/dom.ts';
import {hideToastsFrom} from '../toast.ts';

const fomanticModalFn = $.fn.modal;

// use our own `$.fn.modal` to patch Fomantic's modal module
export function initAriaModalPatch() {
  if ($.fn.modal === ariaModalFn) throw new Error('initAriaModalPatch could only be called once');
  $.fn.modal = ariaModalFn;
  (ariaModalFn as FomanticInitFunction).settings = fomanticModalFn.settings;
  $.fn.fomanticExt.onModalBeforeHidden = onModalBeforeHidden;
  $.fn.modal.settings.onApprove = onModalApproveDefault;
}

// the patched `$.fn.modal` modal function
// * it does the one-time attaching on the first call
function ariaModalFn(this: any, ...args: Parameters<FomanticInitFunction>) {
  const ret = fomanticModalFn.apply(this, args);
  if (args[0] === 'show' || args[0]?.autoShow) {
    for (const el of this) {
      // If there is a form in the modal, there might be a "cancel" button before "ok" button (all buttons are "type=submit" by default).
      // In such case, the "Enter" key will trigger the "cancel" button instead of "ok" button, then the dialog will be closed.
      // It breaks the user experience - the "Enter" key should confirm the dialog and submit the form.
      // So, all "cancel" buttons without "[type]" must be marked as "type=button".
      for (const button of el.querySelectorAll('form button.cancel:not([type])')) {
        button.setAttribute('type', 'button');
      }
    }
  }
  return ret;
}

function onModalBeforeHidden(this: any) {
  const $modal = $(this);
  const elModal = $modal[0];
  hideToastsFrom(elModal.closest('.ui.dimmer') ?? document.body);

  // reset the form after the modal is hidden, after other modal events and handlers (e.g. "onApprove", form submit)
  setTimeout(() => {
    queryElems(elModal, 'form', (form: HTMLFormElement) => form.reset());
  }, 0);
}

function onModalApproveDefault(this: any) {
  const $modal = $(this);
  const selectors = $modal.modal('setting', 'selector');
  const elModal = $modal[0];
  const elApprove = elModal.querySelector(selectors.approve);
  const elForm = elApprove?.closest('form');
  if (!elForm) return true; // no form, just allow closing the modal

  // "form-fetch-action" can handle network errors gracefully,
  // so keep the modal dialog to make users can re-submit the form if anything wrong happens.
  if (elForm.matches('.form-fetch-action')) return false;

  // There is an abuse for the "modal" + "form" combination, the "Approve" button is a traditional form submit button in the form.
  // Then "approve" and "submit" occur at the same time, the modal will be closed immediately before the form is submitted.
  // So here we prevent the modal from closing automatically by returning false, add the "is-loading" class to the form element.
  elForm.classList.add('is-loading');
  return false;
}
