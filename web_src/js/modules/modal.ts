import {generateElemId, queryElems} from '../utils/dom.ts';
import {hideToastsFrom} from './toast.ts';

type ModalOpts = {
  onApprove?: () => boolean | void, // return false to keep the modal open
  onHidden?: () => void,
};

const modalOpts = new WeakMap<Element, ModalOpts>();

function defaultApprove(elApprove: Element): boolean {
  const elForm = elApprove.closest('form');
  if (!elForm) return true;
  if (!elForm.matches('.form-fetch-action') && elForm.reportValidity()) elForm.classList.add('is-loading'); // keep the modal open while its form submits
  return false;
}

function lockPageScroll(locked: boolean) {
  if (document.body.classList.contains('tw-overflow-hidden') === locked) return;
  document.body.style.marginRight = locked ? `${window.innerWidth - document.documentElement.clientWidth}px` : ''; // keep the page width when its scrollbar disappears
  document.body.classList.toggle('tw-overflow-hidden', locked);
}

function createModalDialog(elModal: Element): HTMLDialogElement {
  const dialog = document.createElement('dialog');
  dialog.className = 'modal-dialog';
  dialog.append(elModal);
  const elLabel = elModal.querySelector(':scope > .header, :scope > .content');
  if (elLabel) dialog.setAttribute('aria-labelledby', elLabel.id ||= generateElemId('_modal_label_'));

  let backdropPressed = false;
  dialog.addEventListener('pointerdown', (event) => { backdropPressed = event.target === dialog });
  dialog.addEventListener('click', (event) => {
    const target = event.target as Element;
    if (target === dialog) {
      if (backdropPressed) dialog.close();
    } else if (target.closest('.close-modal, .actions .cancel')) {
      dialog.close();
    } else {
      const elApprove = target.closest('.actions :is(.approve, .ok)');
      if (!elApprove) return;
      const {onApprove} = modalOpts.get(elModal)!;
      if ((onApprove ? onApprove() : defaultApprove(elApprove)) !== false) dialog.close();
    }
  });
  dialog.addEventListener('close', () => {
    if (dialog.open) return; // re-opened before this event was dispatched
    hideToastsFrom(dialog);
    queryElems(elModal, 'form', (form: HTMLFormElement) => form.reset());
    if (!document.querySelector('.modal-dialog[open]')) lockPageScroll(false);
    modalOpts.get(elModal)!.onHidden?.();
    if (!dialog.contains(elModal)) dialog.remove();
  });
  document.body.append(dialog);
  return dialog;
}

export function showModal(elModal: Element, opts: ModalOpts = {}) {
  const dialog = elModal.parentElement instanceof HTMLDialogElement ? elModal.parentElement : createModalDialog(elModal);
  if (dialog.open) return;
  modalOpts.set(elModal, opts);
  elModal.querySelector('.close-modal')?.setAttribute('aria-label', 'Close');
  queryElems(elModal, 'form button.cancel:not([type])', (button) => button.setAttribute('type', 'button')); // otherwise "Enter" in the form clicks this "cancel" button before the "ok" button
  lockPageScroll(true);
  dialog.showModal();
}

export function hideModal(elModal: Element) {
  if (elModal.parentElement instanceof HTMLDialogElement) elModal.parentElement.close();
}
