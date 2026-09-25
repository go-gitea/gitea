import {userEvent} from 'vitest/browser';
import {createElementFromHTML} from '../utils/dom.ts';
import {availableSizeForPlacement, createTippy} from './tippy.ts';

test('createTippy handles keys in menus only', async () => {
  const menuButton = createElementFromHTML('<button>menu</button>');
  const panelButton = createElementFromHTML('<button>panel</button>');
  document.body.append(menuButton, panelButton);

  const clicked: Array<string> = [];
  const menuContent = createElementFromHTML('<div><button class="item">a</button><button class="item">b</button></div>');
  for (const item of menuContent.querySelectorAll('.item')) item.addEventListener('click', () => { clicked.push(item.textContent) });
  const menu = createTippy(menuButton, {content: menuContent, theme: 'menu', trigger: 'manual', interactive: true});
  menu.show();
  await userEvent.keyboard('{ArrowDown}{ArrowDown} {Enter}');
  expect(clicked).toEqual(['b', 'b']);
  await userEvent.keyboard('{Escape}');
  expect(menu.state.isVisible).toBe(false);
  expect(document.activeElement).toBe(menuButton);

  menu.show();
  const panel = createTippy(panelButton, {content: createElementFromHTML('<div><textarea></textarea></div>'), trigger: 'manual', interactive: true});
  panel.show();
  const textarea = panel.popper.querySelector('textarea')!;
  textarea.focus();
  await userEvent.keyboard('a b{Enter}{ArrowUp}c{Escape}');
  expect(textarea.value).toEqual('ca b\n');
  expect(menu.state.isVisible).toBe(true);
  expect(panel.state.isVisible).toBe(true);

  menu.destroy();
  panel.destroy();
  menuButton.remove();
  panelButton.remove();
});

test('availableSizeForPlacement', () => {
  const rect = (values: Partial<DOMRect>) => values as DOMRect;
  vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(1000);
  vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(800);

  // on the placement axis it is the gap to the edge the popup opens towards, the viewport on the other
  expect(availableSizeForPlacement(rect({top: 400, bottom: 432}), 'bottom-end', 0)).toEqual({width: 784, height: 560});
  expect(availableSizeForPlacement(rect({top: 400, bottom: 432}), 'top-end', 0)).toEqual({width: 784, height: 392});
  expect(availableSizeForPlacement(rect({left: 100, right: 300}), 'right', 0)).toEqual({width: 492, height: 984});
  expect(availableSizeForPlacement(rect({left: 100, right: 300}), 'left', 0)).toEqual({width: 92, height: 984});

  // the placement offset eats into the space on that axis
  expect(availableSizeForPlacement(rect({top: 400, bottom: 432}), 'bottom-end', -6).height).toEqual(554);
});
