function getLuminance(r, g, b) {
  // Reference from: https://firsching.ch/github_labels.html and https://www.w3.org/WAI/GL/wiki/Relative_luminance
  // In the future WCAG 3 APCA may be a better solution.
  const luminance = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
  return luminance;
}

function getRGB(channel){
  const sRGB = channel / 255;
  const res = (sRGB <= 0.03928) ? sRGB / 12.92 : ((sRGB + 0.055) / 1.055) ** 2.4;
  return res;
}

export function isUseLightColor(r, g, b) {
  return getLuminance(r, g, b) < 0.453;
}
