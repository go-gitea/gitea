import {assignElementProperty, type ElementWithAssignableProperties} from './common-button.ts';

test('assignElementProperty', () => {
  const elForm = document.createElement('form');
  assignElementProperty(elForm, 'action', '/test-link');
  expect(elForm.action).contains('/test-link'); // the DOM always returns absolute URL
  expect(elForm.getAttribute('action')).eq('/test-link');
  assignElementProperty(elForm, 'text-content', 'dummy');
  expect(elForm.textContent).toBe('dummy');

  // mock a form with its property "action" overwritten by an input element
  const elFormWithAction = new class implements ElementWithAssignableProperties {
    action = document.createElement('input'); // now "form.action" is not string, but an input element
    _attrs: Record<string, string> = {};
    setAttribute(name: string, value: string) { this._attrs[name] = value }
    getAttribute(name: string): string | null { return this._attrs[name] }
  }();
  assignElementProperty(elFormWithAction, 'action', '/bar');
  expect(elFormWithAction.getAttribute('action')).eq('/bar');

  const elInput = document.createElement('input');
  expect(elInput.readOnly).toBe(false);
  assignElementProperty(elInput, 'read-only', 'true');
  expect(elInput.readOnly).toBe(true);
});
