import {assignElementProperty} from './common-button.ts';

test('assignElementProperty', () => {
  const elForm = document.createElement('form');
  assignElementProperty(elForm, 'action', '/test-link');
  expect(elForm.action).contains('/test-link'); // the DOM always returns absolute URL
  assignElementProperty(elForm, 'text-content', 'dummy');
  expect(elForm.textContent).toBe('dummy');

  const elInput = document.createElement('input');
  expect(elInput.readOnly).toBe(false);
  assignElementProperty(elInput, 'read-only', 'true');
  expect(elInput.readOnly).toBe(true);
});
