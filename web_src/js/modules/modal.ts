import {hideAll} from 'tippy.js';
import {generateElemId, queryElems} from '../utils/dom.ts';
import {hideToastsAll} from './toast.ts';

type ModalOpts = {
  onApprove?: () => boolean | void, // return false to keep the modal open
  onShow?: () => void | Promise<void>,
  onHidden?: () => void,
};

const modalStates = new WeakMap<Element, {opts: ModalOpts, opener: Element | null}>();

const topModal = () => Array.from(document.querySelectorAll<HTMLDialogElement>('.modal-dialog[open]')).at(-1);

function blockPage(elBackdrop?: Element | null) {
  const wasBlocked = document.body.classList.contains('modal-page-blocked');
  const shouldBlock = Boolean(elBackdrop);
  if (wasBlocked !== shouldBlock) {
    // keep the page width when its scrollbar disappears
    document.body.style.marginRight = shouldBlock ? `${window.innerWidth - document.documentElement.clientWidth}px` : '';
    document.body.classList.toggle('modal-page-blocked', shouldBlock);
  }
  for (const el of document.body.children) {
    (el as HTMLElement).inert = shouldBlock && el !== elBackdrop;
  }
}

export function initGlobalModal() {
  // on window, so popups handling Escape on document can consume it first
  window.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || e.defaultPrevented || e.isComposing) return;
    const elModal = topModal();
    if (elModal) hideModal(elModal);
  });
}

function defaultApprove(elApprove: Element): boolean {
  const elForm = elApprove.closest('form');
  if (!elForm) return true;

  // if there is traditional form, keep the modal open while the form submits
  // "form-fetch-action" form's submit (is-loading) is handled by the fetch-action module
  if (!elForm.matches('.form-fetch-action') && elForm.reportValidity()) {
    elForm.classList.add('is-loading');
  }
  return false;
}

function createModalBackdrop(elModal: HTMLDialogElement): HTMLElement {
  const elBackdrop = document.createElement('div');
  elBackdrop.className = 'modal-backdrop';
  elBackdrop.append(elModal);

  const elAriaLabelTarget = elModal.querySelector(':scope > .header, :scope > .content');
  if (elAriaLabelTarget) {
    elAriaLabelTarget.id ||= generateElemId('_modal_label_');
    elModal.setAttribute('aria-labelledby', elAriaLabelTarget.id);
  }

  let backdropPressed = false;
  elBackdrop.addEventListener('pointerdown', (event) => { backdropPressed = event.target === elBackdrop });
  elBackdrop.addEventListener('click', (event) => {
    const target = event.target as Element;
    if (target === elBackdrop) {
      if (backdropPressed) hideModal(elModal);
    } else if (target.closest('.close-modal, .actions .cancel')) {
      hideModal(elModal);
    } else {
      const elApprove = target.closest('.actions :is(.approve, .ok)');
      if (!elApprove) return;
      const {onApprove} = modalStates.get(elModal)!.opts;
      if ((onApprove ? onApprove() : defaultApprove(elApprove)) !== false) hideModal(elModal);
    }
  });
  document.body.append(elBackdrop);
  return elBackdrop;
}

export function showModal(elModal: HTMLDialogElement, opts: ModalOpts = {}) {
  if (elModal.open) return;

  const elBackdrop = elModal.parentElement?.matches('.modal-backdrop') ? elModal.parentElement : createModalBackdrop(elModal);
  modalStates.set(elModal, {opts, opener: document.activeElement});
  elModal.querySelector('.close-modal')?.setAttribute('aria-label', 'Close');

  // Avoid "cancel" buttons submitting the form, otherwise "Enter" in the form clicks this "cancel" button before the "ok" button
  queryElems(elModal, 'form button.cancel:not([type])', (button) => button.setAttribute('type', 'button'));

  hideToastsAll();
  hideAll();
  blockPage(elBackdrop);

  elModal.show(); // not showModal: popups and toasts mounted on body must stay usable above it, see https://github.com/whatwg/html/issues/9936
  opts.onShow?.();
}

export function hideModal(elModal: HTMLDialogElement) {
  if (!elModal.open) return;
  const {opts, opener} = modalStates.get(elModal)!;
  const elBackdrop = elModal.parentElement!;
  elModal.close();
  blockPage(topModal()?.parentElement);

  // don't leave the focus in the closed dialog when the opener is gone
  if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
  if (opener instanceof HTMLElement) opener.focus();

  hideToastsAll();
  queryElems(elModal, 'form', (form: HTMLFormElement) => form.reset());
  opts.onHidden?.();
  if (!elBackdrop.contains(elModal)) elBackdrop.remove();
}
