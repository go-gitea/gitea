import {userEvent} from 'vitest/browser';
import {createElementFromHTML} from '../utils/dom.ts';
import {hideModal, initGlobalModal, showModal} from './modal.ts';
import {showErrorToast} from './toast.ts';

test('showModal blocks the page and restores focus until closed by actions, backdrop or unconsumed Escape on the topmost modal', async () => {
  initGlobalModal();
  document.body.innerHTML = `
    <button id="opener">Opener</button>
    <dialog class="modal-dialog">
      <div class="header">Header <button type="button" class="close-modal"></button></div>
      <form class="content">
        <input name="text">
        <div class="actions"><button class="ui cancel button">Cancel</button><button type="button" class="ui ok button">OK</button></div>
      </form>
    </dialog>
  `;
  const opener = document.querySelector<HTMLElement>('#opener')!;
  const modal = document.querySelector<HTMLDialogElement>('.modal-dialog')!;
  const onHidden = vi.fn();
  let approve = false;
  opener.focus();
  showModal(modal, {onApprove: () => approve, onHidden});
  const backdrop = modal.parentElement!;
  expect(opener.inert).toBe(true);
  expect(backdrop.inert).toBe(false);
  expect(modal.getAttribute('aria-labelledby')).toBe(modal.querySelector('.header')!.id);
  expect(document.body.classList.contains('modal-page-blocked')).toBe(true);
  expect(modal.querySelector('.cancel')!.getAttribute('type')).toBe('button');

  modal.querySelector('input')!.value = 'changed';
  modal.querySelector<HTMLElement>('.ok')!.click();
  expect(modal.open).toBe(true);
  approve = true;
  modal.querySelector<HTMLElement>('.ok')!.click();
  expect(onHidden).toHaveBeenCalledOnce();
  expect(modal.querySelector('input')!.value).toBe('');
  expect(document.body.classList.contains('modal-page-blocked')).toBe(false);
  expect(opener.inert).toBe(false);
  expect(document.activeElement).toBe(opener);

  for (const selector of ['.cancel', '.close-modal']) {
    showModal(modal);
    modal.querySelector<HTMLElement>(selector)!.click();
    expect(modal.open).toBe(false);
  }

  showModal(modal);
  await userEvent.click(modal.querySelector('input')!);
  expect(modal.open).toBe(true);
  backdrop.dispatchEvent(new PointerEvent('pointerdown'));
  backdrop.click();
  expect(modal.open).toBe(false);

  showModal(modal);
  document.addEventListener('keydown', (e) => e.preventDefault(), {once: true});
  await userEvent.keyboard('{Escape}');
  expect(modal.open).toBe(true);

  const nested = createElementFromHTML<HTMLDialogElement>('<dialog class="modal-dialog"></dialog>');
  showModal(nested, {onHidden: () => nested.remove()});
  const nestedBackdrop = nested.parentElement!;
  expect(backdrop.inert).toBe(true);
  await userEvent.keyboard('{Escape}');
  expect(nestedBackdrop.isConnected).toBe(false);
  expect(modal.open).toBe(true);
  expect(backdrop.inert).toBe(false);
  expect(opener.inert).toBe(true);
  expect(modal.contains(document.activeElement)).toBe(true);

  showErrorToast('error in modal');
  expect(document.querySelector('.toastify.on')).not.toBeNull();
  await userEvent.keyboard('{Escape}');
  expect(modal.open).toBe(false);
  expect(document.querySelector('.toastify.on')).toBeNull();

  (document.activeElement as HTMLElement).blur();
  showModal(modal);
  hideModal(modal);
  expect(document.activeElement).toBe(document.body);

  showModal(modal, {onHidden: () => modal.remove()});
  hideModal(modal);
  opener.focus();
  expect(backdrop.isConnected).toBe(false);
  expect(document.activeElement).toBe(opener);
});
