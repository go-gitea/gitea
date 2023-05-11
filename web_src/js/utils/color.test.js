import {test, expect} from 'vitest';
import {hexToRGBColor, useLightTextOnBackground} from './color.js';

test('hexToRGBColor', () => {
  expect(hexToRGBColor('2b8685')).toEqual([43, 134, 133]);
  expect(hexToRGBColor('1e1')).toEqual([17, 238, 17]);
  expect(hexToRGBColor('#1e1')).toEqual([17, 238, 17]);
  expect(hexToRGBColor('1e16')).toEqual([17, 238, 17]);
  expect(hexToRGBColor('3bb6b3')).toEqual([59, 182, 179]);
  expect(hexToRGBColor('#3bb6b399')).toEqual([59, 182, 179]);
  expect(hexToRGBColor('#0')).toEqual([0, 0, 0]);
  expect(hexToRGBColor('#00000')).toEqual([0, 0, 0]);
  expect(hexToRGBColor('#1234567')).toEqual([0, 0, 0]);
});

test('useLightTextOnBackground', () => {
  expect(useLightTextOnBackground(215, 58, 74)).toBe(true);
  expect(useLightTextOnBackground(0, 117, 202)).toBe(true);
  expect(useLightTextOnBackground(207, 211, 215)).toBe(false);
  expect(useLightTextOnBackground(162, 238, 239)).toBe(false);
  expect(useLightTextOnBackground(112, 87, 255)).toBe(true);
  expect(useLightTextOnBackground(0, 134, 114)).toBe(true);
  expect(useLightTextOnBackground(228, 230, 105)).toBe(false);
  expect(useLightTextOnBackground(216, 118, 227)).toBe(true);
  expect(useLightTextOnBackground(255, 255, 255)).toBe(false);
  expect(useLightTextOnBackground(43, 134, 133)).toBe(true);
  expect(useLightTextOnBackground(43, 135, 134)).toBe(true);
  expect(useLightTextOnBackground(44, 135, 134)).toBe(true);
  expect(useLightTextOnBackground(59, 182, 179)).toBe(true);
  expect(useLightTextOnBackground(124, 114, 104)).toBe(true);
  expect(useLightTextOnBackground(126, 113, 108)).toBe(true);
  expect(useLightTextOnBackground(129, 112, 109)).toBe(true);
  expect(useLightTextOnBackground(128, 112, 112)).toBe(true);
});
