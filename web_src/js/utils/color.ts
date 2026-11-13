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

/** Resolves any CSS color, including var() and color-mix(), to hex for consumers that only parse hex or rgb(). */
export function resolveCssColor(color: string): string {
  const probe = document.createElement('div');
  probe.style.color = `color-mix(in srgb, ${color} 100%, transparent)`;
  document.documentElement.append(probe);
  const computed = getComputedStyle(probe).color; // "color(srgb r g b / a)" with 0..1 channels, or "rgb()" in some engines
  probe.remove();
  const [r, g, b, a = 1] = computed.match(/-?[\d.]+(?:e-?\d+)?/g)!.map(Number);
  const scale = computed.startsWith('color(') ? 255 : 1;
  return colord({r: r * scale, g: g * scale, b: b * scale, a}).toHex();
}

const chartColorVars = {
  text: 'text',
  border: 'secondary-alpha-60',
  commits: 'primary-alpha-60',
  additions: 'green',
  deletions: 'red',
};

export function chartJsColor(name: keyof typeof chartColorVars): string {
  return resolveCssColor(`var(--color-${chartColorVars[name]})`);
}
