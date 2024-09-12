import tinycolor from 'tinycolor2';
import type {ColorInput} from 'tinycolor2';

// Returns relative luminance for a SRGB color - https://en.wikipedia.org/wiki/Relative_luminance
// Keep this in sync with modules/util/color.go
function getRelativeLuminance(color: ColorInput): number {
  const {r, g, b} = tinycolor(color).toRgb();
  return (0.2126729 * r + 0.7151522 * g + 0.072175 * b) / 255;
}

function useLightText(backgroundColor: ColorInput): boolean {
  return getRelativeLuminance(backgroundColor) < 0.453;
}

// Given a background color, returns a black or white foreground color that the highest
// contrast ratio. In the future, the APCA contrast function, or CSS `contrast-color` will be better.
// https://github.com/color-js/color.js/blob/eb7b53f7a13bb716ec8b28c7a56f052cd599acd9/src/contrast/APCA.js#L42
export function contrastColor(backgroundColor: ColorInput): string {
  return useLightText(backgroundColor) ? '#fff' : '#000';
}

function resolveColors(obj: Record<string, string>): Record<string, string> {
  const styles = window.getComputedStyle(document.documentElement);
  const getColor = (name: string) => styles.getPropertyValue(name).trim();
  return Object.fromEntries(Object.entries(obj).map(([key, value]) => [key, getColor(value)]));
}

export const chartJsColors = resolveColors({
  text: '--color-text',
  border: '--color-secondary-alpha-60',
  commits: '--color-primary-alpha-60',
  additions: '--color-green',
  deletions: '--color-red',
});
