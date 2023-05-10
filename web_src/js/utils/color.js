// Check similar implementation in modules/util/color.go and keep synchronization
// Return R, G, B values defined in reletive luminance
function getLuminanceRGB(channel) {
  const sRGB = channel / 255;
  return (sRGB <= 0.03928) ? sRGB / 12.92 : ((sRGB + 0.055) / 1.055) ** 2.4;
}

// Reference from: https://www.w3.org/WAI/GL/wiki/Relative_luminance
function getLuminance(r, g, b) {
  const R = getLuminanceRGB(r);
  const G = getLuminanceRGB(g);
  const B = getLuminanceRGB(b);
  return 0.2126 * R + 0.7152 * G + 0.0722 * B;
}

// Get color as RGB values in 0..255 range from the hex color string (with or without #)
export function hexToRGBColor(backgroundColorStr) {
  let backgroundColor = backgroundColorStr;
  if (backgroundColorStr[0] === '#') {
    backgroundColor = backgroundColorStr.substring(1);
  }
  // only support transfer of rgb, rgba, rrggbb and rrggbbaa
  // if not in these formats, use default values 0, 0, 0
  if (![3, 4, 6, 8].includes(backgroundColor.length)) {
    return [0, 0, 0];
  }
  if ([3, 4].includes(backgroundColor.length)) {
    const [r, g, b] = backgroundColor;
    backgroundColor = `${r}${r}${g}${g}${b}${b}`;
  }
  const r = parseInt(backgroundColor.substring(0, 2), 16);
  const g = parseInt(backgroundColor.substring(2, 4), 16);
  const b = parseInt(backgroundColor.substring(4, 6), 16);
  return [r, g, b];
}

// Reference from: https://firsching.ch/github_labels.html
// In the future WCAG 3 APCA may be a better solution.
// Check if text should use light color based on RGB of background
export function useLightTextOnBackground(r, g, b) {
  return getLuminance(r, g, b) < 0.453;
}
