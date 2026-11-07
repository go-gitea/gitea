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
  const filter = menu.popper.querySelector('input')!;
  const focus = vi.spyOn(filter, 'focus').mockImplementation(() => {});
  menu.show();
  await menu.popperInstance!.update();
  expect(focus).toHaveBeenCalledOnce();
  focus.mockRestore();
  await userEvent.type(filter, 'a b{ArrowLeft}c{Enter}');
  expect(filter.value).toEqual('a cb');
  expect(menu.state.isVisible).toBe(true);

  app.unmount();
  root.remove();
});
