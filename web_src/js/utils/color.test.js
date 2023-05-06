import {test, expect} from 'vitest';
import {isUseLightColor} from './color.js';

test('isUseLightColor', () => {
  expect(isUseLightColor(215, 58, 74)).toBe(true);
  expect(isUseLightColor(0, 117, 202)).toBe(true);
  expect(isUseLightColor(207, 211, 215)).toBe(false);
  expect(isUseLightColor(162, 238, 239)).toBe(false);
  expect(isUseLightColor(112, 87, 255)).toBe(true);
  expect(isUseLightColor(0, 134, 114)).toBe(true);
  expect(isUseLightColor(228, 230, 105)).toBe(false);
  expect(isUseLightColor(216, 118, 227)).toBe(false);
  expect(isUseLightColor(255, 255, 255)).toBe(false);
  expect(isUseLightColor(43, 134, 133)).toBe(true);
  expect(isUseLightColor(43, 135, 134)).toBe(true);
  expect(isUseLightColor(44, 135, 134)).toBe(false);
  expect(isUseLightColor(59, 182, 179)).toBe(false);
  expect(isUseLightColor(124, 114, 104)).toBe(true);
  expect(isUseLightColor(126, 113, 108)).toBe(true);
  expect(isUseLightColor(129, 112, 109)).toBe(true);
  expect(isUseLightColor(128, 112, 112)).toBe(true);
});
