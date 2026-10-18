import {userEvent} from 'vitest/browser';
import {hideModal, showModal} from './modal.ts';

test('showModal closes by actions, backdrop and Escape unless approval is vetoed, and cleans up when closed', async () => {
  document.body.innerHTML = `
    <div class="ui modal">
      <div class="header">Header <button type="button" class="close-modal"></button></div>
      <form class="content">
        <input name="text">
        <div class="actions"><button class="ui cancel button">Cancel</button><button type="button" class="ui ok button">OK</button></div>
      </form>
    </div>
  `;
  const modal = document.querySelector<HTMLElement>('.ui.modal')!;
  const onHidden = vi.fn();
  let approve = false;
  showModal(modal, {onApprove: () => approve, onHidden});
  const dialog = modal.parentElement as HTMLDialogElement;
  const closed = () => new Promise((resolve) => dialog.addEventListener('close', resolve, {once: true}));
  expect(dialog.matches(':modal')).toBe(true);
  expect(dialog.getAttribute('aria-labelledby')).toBe(modal.querySelector('.header')!.id);
  expect(document.body.classList.contains('tw-overflow-hidden')).toBe(true);
  expect(modal.querySelector('.cancel')!.getAttribute('type')).toBe('button');

  modal.querySelector('input')!.value = 'changed';
  modal.querySelector<HTMLElement>('.ok')!.click();
  expect(dialog.open).toBe(true);
  approve = true;
  let close = closed();
  modal.querySelector<HTMLElement>('.ok')!.click();
  await close;
  expect(onHidden).toHaveBeenCalledOnce();
  expect(modal.querySelector('input')!.value).toBe('');
  expect(document.body.classList.contains('tw-overflow-hidden')).toBe(false);

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
  await userEvent.keyboard('{Escape}');
  expect(dialog.open).toBe(false);

  showModal(modal, {onHidden: () => modal.remove()});
  close = closed();
  hideModal(modal);
  await close;
  expect(dialog.isConnected).toBe(false);
});
