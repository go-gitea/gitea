import './external-render-helper.ts';

test('isValidCssColor', async () => {
  const isValidCssColor = window.testModules.externalRenderHelper!.isValidCssColor;
  expect(isValidCssColor(null)).toBe(false);
  expect(isValidCssColor('')).toBe(false);

  expect(isValidCssColor('#123')).toBe(true);
  expect(isValidCssColor('#1234')).toBe(true);
  expect(isValidCssColor('#abcabc')).toBe(true);
  expect(isValidCssColor('#abcabc12')).toBe(true);

  expect(isValidCssColor('rgb(255 255 255)')).toBe(true);
  expect(isValidCssColor('rgb(0, 255, 255)')).toBe(true);

  // examples from MDN: https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Values/color_value/rgb
  expect(isValidCssColor('rgb(255 255 255 / 50%)')).toBe(true);
  expect(isValidCssColor('rgb(from #123456 hwb(120deg 10% 20%) calc(g + 40) b / 0.5)')).toBe(true);

  expect(isValidCssColor('#123 ; other')).toBe(false);
  expect(isValidCssColor('#123 : other')).toBe(false);
  expect(isValidCssColor('#rgb(0, 255, 255); other')).toBe(false);
  expect(isValidCssColor('#rgb(0, 255, 255)} other')).toBe(false);
  expect(isValidCssColor('url(other)')).toBe(false);
});
