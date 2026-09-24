import {availableSizeForPlacement} from './tippy.ts';

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
