import {contrastColor} from './color.ts';

test('contrastColor', () => {
  expect(contrastColor('#d73a4a')).toBe('#fff');
  expect(contrastColor('#0075ca')).toBe('#fff');
  expect(contrastColor('#cfd3d7')).toBe('#000');
  expect(contrastColor('#a2eeef')).toBe('#000');
  expect(contrastColor('#7057ff')).toBe('#fff');
  expect(contrastColor('#008672')).toBe('#fff');
  expect(contrastColor('#e4e669')).toBe('#000');
  expect(contrastColor('#d876e3')).toBe('#000');
  expect(contrastColor('#ffffff')).toBe('#000');
  expect(contrastColor('#2b8684')).toBe('#fff');
  expect(contrastColor('#2b8786')).toBe('#fff');
  expect(contrastColor('#2c8786')).toBe('#000');
  expect(contrastColor('#3bb6b3')).toBe('#000');
  expect(contrastColor('#7c7268')).toBe('#fff');
  expect(contrastColor('#7e716c')).toBe('#fff');
  expect(contrastColor('#81706d')).toBe('#fff');
  expect(contrastColor('#807070')).toBe('#fff');
  expect(contrastColor('#84b6eb')).toBe('#000');
});
