// Check similar implementation in modules/util/color.go and keep synchronization
// Return R, G, B values defined in reletive luminance
function getLuminanceRGB(channel) {
  const sRGB = channel / 255;
  const res = (sRGB <= 0.03928) ? sRGB / 12.92 : ((sRGB + 0.055) / 1.055) ** 2.4;
  return res;
}

// Reference from: https://www.w3.org/WAI/GL/wiki/Relative_luminance
function getLuminance(r, g, b) {
  const R = getLuminanceRGB(r);
  const G = getLuminanceRGB(g);
  const B = getLuminanceRGB(b);
  const luminance = 0.2126 * R + 0.7152 * G + 0.0722 * B;
  return luminance;
}

const getChunksFromString = (st, chunkSize) => st.match(new RegExp(`.{${chunkSize}}`, 'g'));

const convertHexUnitTo256 = (hexStr) => parseInt(hexStr.repeat(2 / hexStr.length), 16);

const getAlphafloat = (a) => {
  if (a !== undefined) {
    return a / 255;
  }
  return 1;
};

// Get color as RGB values in 0..255 range from the hex color string (with or without #)
export function hexToRGBColor(backgroundColorStr, ignoreAlpha = true) {
  let backgroundColor = backgroundColorStr;
  if (backgroundColorStr[0] === '#') {
    backgroundColor = backgroundColorStr.substring(1);
  }
  // only support transfer of rgb, rgba, rrggbb, and rrggbbaa
  // if not in this format, use default values 0, 0, 0 or 0, 0, 0, 1
  if (![3, 4, 6, 8].includes(backgroundColor.length)) {
    return ignoreAlpha ? [0, 0, 0] : [0, 0, 0, 1];
  }
  const chunkSize = Math.floor(backgroundColor.length / 3);
  const hexArr = getChunksFromString(backgroundColor, chunkSize);
  const [r, g, b, a] = hexArr.map(convertHexUnitTo256);
  return ignoreAlpha ? [r, g, b] : [r, g, b, getAlphafloat(a)];
}

// Reference from: https://firsching.ch/github_labels.html
// In the future WCAG 3 APCA may be a better solution.
// Check if text should use light color based on RGB of background
export function useLightTextOnBackground(r, g, b) {
  return getLuminance(r, g, b) < 0.453;
}
