import tinycolor from 'tinycolor2';

// Returns relative luminance for a SRGB color - https://en.wikipedia.org/wiki/Relative_luminance
// Keep this in sync with modules/util/color.go
function getRelativeLuminance(color) {
  const {r, g, b} = tinycolor(color).toRgb();
  return (0.2126729 * r + 0.7151522 * g + 0.072175 * b) / 255;
}

function useLightText(backgroundColor) {
  return getRelativeLuminance(backgroundColor) < 0.453 ? true : false;
}

export function contrastColor(backgroundColor) {
  return useLightText(backgroundColor) ? '#fff' : '#000';
}

function resolveColors(obj) {
  const styles = window.getComputedStyle(document.documentElement);
  const getColor = (name) => styles.getPropertyValue(name).trim();
  return Object.fromEntries(Object.entries(obj).map(([key, value]) => [key, getColor(value)]));
}

export const chartJsColors = resolveColors({
  text: '--color-text',
  border: '--color-secondary-alpha-60',
  commits: '--color-primary-alpha-60',
  additions: '--color-green',
  deletions: '--color-red',
});

function hex(x) {
  const hexDigits = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'];
  return Number.isNaN(x) ? '00' : hexDigits[(x - x % 16) / 16] + hexDigits[x % 16];
}

export function rgbToHex(rgb) {
  rgb = rgb.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+).*\)$/);
  return `#${hex(rgb[1])}${hex(rgb[2])}${hex(rgb[3])}`;
}
