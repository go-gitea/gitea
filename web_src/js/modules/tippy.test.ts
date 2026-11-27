import {createElementFromHTML} from '../utils/dom.ts';
import {availableHeightForPlacement, createTippy} from './tippy.ts';

test('limitSizeToViewport flip', async () => {
  const button = createElementFromHTML('<button style="position: fixed; bottom: 20px">button</button>');
  document.body.append(button);
  const instance = createTippy(button, {
    content: createElementFromHTML('<div style="height: 3000px"></div>'),
    trigger: 'manual',
    placement: 'bottom-start',
    limitSizeToViewport: true,
  });
  instance.show();
  await instance.popperInstance!.update();
  expect(instance.popper.querySelector('.tippy-box')!.getAttribute('data-placement')).toEqual('top-start');
  instance.destroy();
  button.remove();
});

test('availableHeightForPlacement', () => {
  const rect = (values: Partial<DOMRect>) => values as DOMRect;
  vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(1000);

  expect(availableHeightForPlacement(rect({bottom: 432}), 'bottom-end', 0)).toBe(560);
  expect(availableHeightForPlacement(rect({top: 400}), 'top-end', 0)).toBe(392);
  expect(availableHeightForPlacement(rect({}), 'right', 0)).toBe(984);
  expect(availableHeightForPlacement(rect({}), 'left', 0)).toBe(984);
  expect(availableHeightForPlacement(rect({bottom: 432}), 'bottom-end', -6)).toBe(554);
});
