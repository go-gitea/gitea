import {colord} from 'colord';
import type {AnyColor} from 'colord';

/** Undoes the sRGB transfer function, channel is in 0..255 range. */
function linearizeChannel(channel: number): number {
  const srgb = channel / 255;
  return srgb <= 0.04045 ? srgb / 12.92 : ((srgb + 0.055) / 1.055) ** 2.4;
}

/** Returns relative luminance for a SRGB color - https://www.w3.org/TR/WCAG20/#relativeluminancedef */
// Keep this in sync with modules/util/color.go
function getRelativeLuminance(color: AnyColor): number {
  const {r, g, b} = colord(color).toRgb();
  return 0.2126 * linearizeChannel(r) + 0.7152 * linearizeChannel(g) + 0.0722 * linearizeChannel(b);
}

function useLightText(backgroundColor: AnyColor): boolean {
  return getRelativeLuminance(backgroundColor) < 0.36; // matches APCA better than WCAG's own 0.179
}

/** Given a background color, returns a black or white foreground color with the highest contrast ratio. */
export function contrastColor(backgroundColor: AnyColor): string {
  return useLightText(backgroundColor) ? '#fff' : '#000';
}

/** Resolves any CSS color, including color-mix(), to "#rrggbbaa" for consumers that only parse hex or rgb(). */
export function resolveCssColor(color: string): string {
  const ctx = document.createElement('canvas').getContext('2d')!;
  ctx.fillStyle = color;
  ctx.fillRect(0, 0, 1, 1);
  return `#${Array.from(ctx.getImageData(0, 0, 1, 1).data, (channel) => channel.toString(16).padStart(2, '0')).join('')}`;
}

function resolveColors(obj: Record<string, string>): Record<string, string> {
  const styles = window.getComputedStyle(document.documentElement);
  const getColor = (name: string) => resolveCssColor(styles.getPropertyValue(name).trim());
  return Object.fromEntries(Object.entries(obj).map(([key, value]) => [key, getColor(value)]));
}

export const chartJsColors = resolveColors({
  text: '--color-text',
  border: '--color-secondary-alpha-60',
  commits: '--color-primary-alpha-60',
  additions: '--color-green',
  deletions: '--color-red',
});
