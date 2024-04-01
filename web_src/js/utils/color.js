import tinycolor from 'tinycolor2';

// Check similar implementation in modules/util/color.go and keep synchronization
// Return R, G, B values defined in reletive luminance

// Reference from: https://www.w3.org/WAI/GL/wiki/Relative_luminance
function getLuminance(r, g, b) {
  return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
}

// Reference from: https://firsching.ch/github_labels.html
// In the future WCAG 3 APCA may be a better solution.
// Check if text should use light color based on RGB of background
export function contrastColor(color) {
  const {r, g, b} = tinycolor(color).toRgb();
  return getLuminance(r, g, b) < 0.453 ? '#fff' : '#000';
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
