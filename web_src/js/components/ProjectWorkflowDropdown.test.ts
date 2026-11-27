import {createApp} from 'vue';
import {userEvent} from 'vitest/browser';
import ProjectWorkflowDropdown from './ProjectWorkflowDropdown.vue';
import {initDropdown} from '../modules/dropdown.ts';

test('label filter focuses on open, supports spaces and caret movement, and picks the match on Enter', async () => {
  const root = document.createElement('div');
  document.body.append(root);
  const update = vi.fn();
  const app = createApp(ProjectWorkflowDropdown, {
    label: 'Labels', options: [{value: 1, text: 'a cb', html: 'a cb'}], readonly: false, modelValue: [], 'onUpdate:modelValue': update,
  });
  app.mount(root);
  const dropdown = initDropdown(root.querySelector<HTMLElement>('.dropdown-container')!);
  const filter = dropdown.filterInput!;
  onTestFinished(() => {
    dropdown.hide();
    app.unmount();
    root.remove();
  });

  await userEvent.click(dropdown.trigger);
  await expect.poll(() => document.activeElement).toBe(filter);
  await userEvent.keyboard('a b{ArrowLeft}c{Enter}');
  expect(filter.value).toBe('a cb');
  expect(update).toHaveBeenCalledExactlyOnceWith([1]);
  expect(document.activeElement).toBe(filter);
});
