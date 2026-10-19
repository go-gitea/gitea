import {userEvent} from 'vitest/browser';
import {createElementFromHTML} from '../utils/dom.ts';
import {hideModal, showModal} from './modal.ts';
import {showErrorToast} from './toast.ts';

test('showModal blocks the page and restores focus until closed by actions, backdrop or unconsumed Escape on the topmost modal', async () => {
  document.body.innerHTML = `
    <button id="opener">Opener</button>
    <div class="modal">
      <div class="header">Header <button type="button" class="close-modal"></button></div>
      <form class="content">
        <input name="text">
        <div class="actions"><button class="ui cancel button">Cancel</button><button type="button" class="ui ok button">OK</button></div>
      </form>
    </div>
  `;
  const opener = document.querySelector<HTMLElement>('#opener')!;
  const modal = document.querySelector<HTMLElement>('.modal')!;
  const onHidden = vi.fn();
  let approve = false;
  opener.focus();
  showModal(modal, {onApprove: () => approve, onHidden});
  const dialog = modal.parentElement as HTMLDialogElement;
  expect(opener.inert).toBe(true);
  expect(dialog.inert).toBe(false);
  expect(dialog.getAttribute('aria-labelledby')).toBe(modal.querySelector('.header')!.id);
  expect(document.body.classList.contains('tw-overflow-hidden')).toBe(true);
  expect(modal.querySelector('.cancel')!.getAttribute('type')).toBe('button');

  modal.querySelector('input')!.value = 'changed';
  modal.querySelector<HTMLElement>('.ok')!.click();
  expect(dialog.open).toBe(true);
  approve = true;
  modal.querySelector<HTMLElement>('.ok')!.click();
  expect(onHidden).toHaveBeenCalledOnce();
  expect(modal.querySelector('input')!.value).toBe('');
  expect(document.body.classList.contains('tw-overflow-hidden')).toBe(false);
  expect(opener.inert).toBe(false);
  expect(document.activeElement).toBe(opener);

  for (const selector of ['.cancel', '.close-modal']) {
    showModal(modal);
    modal.querySelector<HTMLElement>(selector)!.click();
    expect(dialog.open).toBe(false);
  }

  showModal(modal);
  await userEvent.click(modal.querySelector('input')!);
  expect(dialog.open).toBe(true);
  await userEvent.click(dialog, {position: {x: 5, y: 5}});
  expect(dialog.open).toBe(false);

  showModal(modal);
  document.addEventListener('keydown', (e) => e.preventDefault(), {once: true});
  await userEvent.keyboard('{Escape}');
  expect(dialog.open).toBe(true);

  const nested = createElementFromHTML('<div class="modal"></div>');
  showModal(nested, {onHidden: () => nested.remove()});
  const nestedDialog = nested.parentElement!;
  expect(dialog.inert).toBe(true);
  await userEvent.keyboard('{Escape}');
  expect(nestedDialog.isConnected).toBe(false);
  expect(dialog.open).toBe(true);
  expect(dialog.inert).toBe(false);
  expect(opener.inert).toBe(true);
  expect(dialog.contains(document.activeElement)).toBe(true);

  showErrorToast('error in modal');
  expect(document.querySelector('.toastify.on')).not.toBeNull();
  await userEvent.keyboard('{Escape}');
  expect(dialog.open).toBe(false);
  expect(document.querySelector('.toastify.on')).toBeNull();

  (document.activeElement as HTMLElement).blur();
  showModal(modal);
  hideModal(modal);
  expect(document.activeElement).toBe(document.body);

  showModal(modal, {onHidden: () => modal.remove()});
  hideModal(modal);
  opener.focus();
  expect(dialog.isConnected).toBe(false);
  expect(document.activeElement).toBe(opener);
});
