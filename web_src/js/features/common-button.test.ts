import {assignElementProperty, type ElementWithAssignableProperties, initGlobalButtons} from './common-button.ts';
import {hideModal} from '../modules/modal.ts';

test('assignElementProperty', () => {
  const elForm = document.createElement('form');
  expect(() => assignElementProperty(elForm, 'action', '/test-link')).toThrow();
  assignElementProperty(elForm, 'url', '/test-url');
  expect(elForm.action).contains('/test-url'); // the DOM always returns absolute URL
  expect(elForm.getAttribute('action')).eq('/test-url');
  assignElementProperty(elForm, 'text-content', 'dummy');
  expect(elForm.textContent).toBe('dummy');

  // mock a form with its property "action" overwritten by an input element
  const elFormWithAction = new class implements ElementWithAssignableProperties {
    action = document.createElement('input'); // now "form.action" is not string, but an input element
    _attrs: Record<string, string> = {};
    setAttribute(name: string, value: string) { this._attrs[name] = value }
    getAttribute(name: string): string | null { return this._attrs[name] }
    nodeName = 'FORM';
  }();
  assignElementProperty(elFormWithAction, 'url', '/bar');
  expect(elFormWithAction.getAttribute('action')).eq('/bar');

  const elInput = document.createElement('input');
  expect(elInput.readOnly).toBe(false);
  assignElementProperty(elInput, 'read-only', 'true');
  expect(elInput.readOnly).toBe(true);
});

test('onShowModalClick opens and fills a cloned template dialog', () => {
  document.body.innerHTML = `
    <button class="show-modal" data-modal="#modal-template" data-modal-input.value="filled">Open</button>
    <template id="modal-template">
      <dialog class="modal-dialog"><input value="original"></dialog>
    </template>
  `;
  initGlobalButtons();
  const original = document.querySelector('template')!.content.querySelector('dialog')!;

  document.querySelector('button')!.click();

  const modal = document.querySelector<HTMLDialogElement>('dialog.modal-dialog')!;
  const backdrop = modal.parentElement!;
  expect(modal.open).toBe(true);
  expect(modal).not.toBe(original);
  expect(modal.querySelector('input')!.value).toBe('filled');
  expect(original.querySelector('input')!.value).toBe('original');
  hideModal(modal);
  expect(document.body.contains(modal)).toBe(false);
  expect(document.body.contains(backdrop)).toBe(false);
});
