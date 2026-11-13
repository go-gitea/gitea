import {contrastColor, resolveCssColor} from './color.ts';

test('contrastColor', () => {
  expect(contrastColor('#d73a4a')).toBe('#fff');
  expect(contrastColor('#0075ca')).toBe('#fff');
  expect(contrastColor('#cfd3d7')).toBe('#000');
  expect(contrastColor('#a2eeef')).toBe('#000');
  expect(contrastColor('#7057ff')).toBe('#fff');
  expect(contrastColor('#008672')).toBe('#fff');
  expect(contrastColor('#e4e669')).toBe('#000');
  expect(contrastColor('#d876e3')).toBe('#fff');
  expect(contrastColor('#ffffff')).toBe('#000');
  expect(contrastColor('#2b8684')).toBe('#fff');
  expect(contrastColor('#2b8786')).toBe('#fff');
  expect(contrastColor('#2c8786')).toBe('#fff');
  expect(contrastColor('#2bb3b2')).toBe('#fff');
  expect(contrastColor('#2bb4b3')).toBe('#000');
  expect(contrastColor('#3bb6b3')).toBe('#000');
  expect(contrastColor('#7c7268')).toBe('#fff');
  expect(contrastColor('#7e716c')).toBe('#fff');
  expect(contrastColor('#81706d')).toBe('#fff');
  expect(contrastColor('#807070')).toBe('#fff');
  expect(contrastColor('#84b6eb')).toBe('#000');
});

test('resolveCssColor', () => {
  expect(resolveCssColor('#4183c4')).toBe('#4183c4');
  expect(resolveCssColor('color-mix(in srgb, #ff0000 20%, black)')).toBe('#330000');
  expect(resolveCssColor('color-mix(in srgb, #ffffff 40%, transparent)')).toBe('#ffffff66');
  expect(resolveCssColor('color(srgb 0 0.2 1)')).toBe('#0033ff');
  expect(resolveCssColor('#4183c499')).toBe('#4183c499');
  expect(resolveCssColor('oklab(1 0 0)')).toBe('#ffffff');
});
