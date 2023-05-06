// Reference from: https://www.w3.org/WAI/GL/wiki/Relative_luminance and https://firsching.ch/github_labels.html
// In the future WCAG 3 APCA may be a better solution.
function getLuminance(r, g, b) {
  const luminance = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
  return luminance;
}

function _getLuminanceRGB(channel) {
  const sRGB = channel / 255;
  const res = (sRGB <= 0.03928) ? sRGB / 12.92 : ((sRGB + 0.055) / 1.055) ** 2.4;
  return res;
}

// Get rgb channel integers from color string
export function getRGB(backgroundColor) {
  const r = parseInt(backgroundColor.substring(0, 2), 16);
  const g = parseInt(backgroundColor.substring(2, 4), 16);
  const b = parseInt(backgroundColor.substring(4, 6), 16);
  return [r, g, b];
}

export function isUseLightColor(r, g, b) {
  return getLuminance(r, g, b) < 0.453;
}
