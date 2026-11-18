import {userEvent} from 'vitest/browser';
import {createElementFromHTML} from '../utils/dom.ts';
import {availableSizeForPlacement, createTippy} from './tippy.ts';
import tippyStyles from '../../css/modules/tippy.css?inline';

afterEach(() => vi.restoreAllMocks());

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

test('limited popups use the larger side and reposition after content changes', async () => {
  const style = document.createElement('style');
  style.textContent = tippyStyles;
  document.head.append(style);
  const button = createElementFromHTML<HTMLButtonElement>('<button style="position: fixed; left: 100px; width: 100px; height: 32px">Open</button>');
  button.style.top = `${Math.floor(window.innerHeight * 0.65)}px`;
  document.body.append(button);
  const content = createElementFromHTML<HTMLElement>('<div style="width: 240px">Content</div>');
  content.style.height = `${window.innerHeight * 2}px`;
  const instance = createTippy(button, {
    content, placement: 'bottom-end', trigger: 'manual', interactive: true,
    arrow: false, limitSizeToViewport: {vertical: true},
  });
  try {
    instance.show();
    await instance.popperInstance!.update();
    const box = instance.popper.querySelector<HTMLElement>('.tippy-box')!;
    const scroll = instance.popper.querySelector<HTMLElement>('.tippy-content')!;
    expect(box.getBoundingClientRect().top).toBeGreaterThanOrEqual(0);
    expect(box.getBoundingClientRect().bottom).toBeLessThan(button.getBoundingClientRect().top);
    expect(scroll.scrollHeight).toBeGreaterThan(scroll.clientHeight);
    scroll.scrollTop = scroll.scrollHeight;
    expect(scroll.scrollTop).toBeGreaterThan(0);

    content.style.height = '30px';
    await instance.popperInstance!.update();
    expect(box.getBoundingClientRect().top).toBeGreaterThan(button.getBoundingClientRect().bottom);
    expect(box.getBoundingClientRect().bottom).toBeLessThanOrEqual(window.innerHeight);

    content.style.height = `${window.innerHeight * 2}px`;
    await instance.popperInstance!.update();
    expect(box.getBoundingClientRect().top).toBeGreaterThanOrEqual(0);
    expect(box.getBoundingClientRect().bottom).toBeLessThan(button.getBoundingClientRect().top);
  } finally {
    instance.destroy();
    button.remove();
    style.remove();
  }
});
