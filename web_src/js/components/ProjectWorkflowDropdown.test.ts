import {createApp, nextTick} from 'vue';
import {userEvent} from 'vitest/browser';
import ProjectWorkflowDropdown from './ProjectWorkflowDropdown.vue';

test('label filter gets focus on open and keeps space, enter and left/right inside the menu', async () => {
  const root = document.createElement('div');
  document.body.append(root);
  const app = createApp(ProjectWorkflowDropdown, {label: 'Labels', options: [{value: 1, text: 'bug', html: 'bug'}], readonly: false, modelValue: []});
  app.mount(root);
  await nextTick();
  const menu = root.querySelector('button')!._tippy;
  menu.show();
  const filter = menu.popper.querySelector('input')!;
  await expect.poll(() => document.activeElement).toBe(filter);
  await userEvent.keyboard('a b{ArrowLeft}c{Enter}');
  expect(filter.value).toEqual('a cb');
  expect(menu.state.isVisible).toBe(true);

  app.unmount();
  root.remove();
});
