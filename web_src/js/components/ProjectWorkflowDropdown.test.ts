import {createApp, nextTick} from 'vue';
import {userEvent} from 'vitest/browser';
import ProjectWorkflowDropdown from './ProjectWorkflowDropdown.vue';
import {getFloatingElement} from '../modules/floating.ts';

test('label filter gets focus on open and keeps space, enter and left/right inside the menu', async () => {
  const root = document.createElement('div');
  document.body.append(root);
  const app = createApp(ProjectWorkflowDropdown, {label: 'Labels', options: [{value: 1, text: 'bug', html: 'bug'}], readonly: false, modelValue: []});
  app.mount(root);
  await nextTick();
  const menu = getFloatingElement(root.querySelector('button')!)!;
  menu.show();
  const filter = menu.element.querySelector('input')!;
  await expect.poll(() => document.activeElement).toBe(filter);
  await userEvent.keyboard('a b{ArrowLeft}c{Enter}');
  expect(filter.value).toEqual('a cb');
  expect(menu.state.isShown).toBe(true);

  app.unmount();
  root.remove();
});
