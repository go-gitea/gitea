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

// Reference from: https://firsching.ch/github_labels.html
// In the future WCAG 3 APCA may be a better solution.
// Check if text should use light color based on RGB of background
export function useLightTextOnBackground(r, g, b) {
  return getLuminance(r, g, b) < 0.453;
}
