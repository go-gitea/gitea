import {test, expect} from 'vitest';
import {getRGBColor, isUseLightColor} from './color.js';

test('getRGBColor', () => {
  expect(getRGBColor('2b8685')).toEqual([43, 134, 133]);
  expect(getRGBColor('2b8786')).toEqual([43, 135, 134]);
  expect(getRGBColor('2c8786')).toEqual([44, 135, 134]);
  expect(getRGBColor('3bb6b3')).toEqual([59, 182, 179]);
  expect(getRGBColor('7c7268')).toEqual([124, 114, 104]);
  expect(getRGBColor('#7e716c')).toEqual([126, 113, 108]);
  expect(getRGBColor('#807070')).toEqual([128, 112, 112]);
  expect(getRGBColor('#81706d')).toEqual([129, 112, 109]);
  expect(getRGBColor('#d73a4a')).toEqual([215, 58, 74]);
  expect(getRGBColor('#0075ca')).toEqual([0, 117, 202]);
});

test('isUseLightColor', () => {
  expect(isUseLightColor(215, 58, 74)).toBe(true);
  expect(isUseLightColor(0, 117, 202)).toBe(true);
  expect(isUseLightColor(207, 211, 215)).toBe(false);
  expect(isUseLightColor(162, 238, 239)).toBe(false);
  expect(isUseLightColor(112, 87, 255)).toBe(true);
  expect(isUseLightColor(0, 134, 114)).toBe(true);
  expect(isUseLightColor(228, 230, 105)).toBe(false);
  expect(isUseLightColor(216, 118, 227)).toBe(true);
  expect(isUseLightColor(255, 255, 255)).toBe(false);
  expect(isUseLightColor(43, 134, 133)).toBe(true);
  expect(isUseLightColor(43, 135, 134)).toBe(true);
  expect(isUseLightColor(44, 135, 134)).toBe(true);
  expect(isUseLightColor(59, 182, 179)).toBe(true);
  expect(isUseLightColor(124, 114, 104)).toBe(true);
  expect(isUseLightColor(126, 113, 108)).toBe(true);
  expect(isUseLightColor(129, 112, 109)).toBe(true);
  expect(isUseLightColor(128, 112, 112)).toBe(true);
});
