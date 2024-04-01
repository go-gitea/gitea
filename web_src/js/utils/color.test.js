import {contrastColor} from './color.js';

test('contrastColor', () => {
  expect(contrastColor(215, 58, 74)).toBe(true);
  expect(contrastColor(0, 117, 202)).toBe(true);
  expect(contrastColor(207, 211, 215)).toBe(false);
  expect(contrastColor(162, 238, 239)).toBe(false);
  expect(contrastColor(112, 87, 255)).toBe(true);
  expect(contrastColor(0, 134, 114)).toBe(true);
  expect(contrastColor(228, 230, 105)).toBe(false);
  // expect(contrastColor(216, 118, 227)).toBe(true);
  expect(contrastColor(255, 255, 255)).toBe(false);
  expect(contrastColor(43, 134, 133)).toBe(true);
  expect(contrastColor(43, 135, 134)).toBe(true);
  // expect(contrastColor(44, 135, 134)).toBe(true);
  // expect(contrastColor(59, 182, 179)).toBe(true);
  expect(contrastColor(124, 114, 104)).toBe(true);
  expect(contrastColor(126, 113, 108)).toBe(true);
  expect(contrastColor(129, 112, 109)).toBe(true);
  expect(contrastColor(128, 112, 112)).toBe(true);
  expect(contrastColor('#84b6eb')).toBe(true);
});
