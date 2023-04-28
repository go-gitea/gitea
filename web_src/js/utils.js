// transform /path/to/file.ext to file.ext
export function basename(path = '') {
  return path ? path.replace(/^.*\//, '') : '';
}

// transform /path/to/file.ext to .ext
export function extname(path = '') {
  const [_, ext] = /.+(\.[^.]+)$/.exec(path) || [];
  return ext || '';
}

// join a list of path segments with slashes, ensuring no double slashes
export function joinPaths(...parts) {
  let str = '';
  for (const part of parts) {
    if (!part) continue;
    str = !str ? part : `${str.replace(/\/$/, '')}/${part.replace(/^\//, '')}`;
  }
  return str;
}

// test whether a variable is an object
export function isObject(obj) {
  return Object.prototype.toString.call(obj) === '[object Object]';
}

// returns whether a dark theme is enabled
export function isDarkTheme() {
  const style = window.getComputedStyle(document.documentElement);
  return style.getPropertyValue('--is-dark-theme').trim().toLowerCase() === 'true';
}

// strip <tags> from a string
export function stripTags(text) {
  return text.replace(/<[^>]*>?/g, '');
}

// searches the inclusive range [minValue, maxValue].
// credits: https://matthiasott.com/notes/write-your-media-queries-in-pixels-not-ems
export function mqBinarySearch(feature, minValue, maxValue, step, unit) {
  if (maxValue - minValue < step) {
    return minValue;
  }
  const mid = Math.ceil((minValue + maxValue) / 2 / step) * step;
  if (matchMedia(`screen and (min-${feature}:${mid}${unit})`).matches) {
    return mqBinarySearch(feature, mid, maxValue, step, unit); // feature is >= mid
  }
  return mqBinarySearch(feature, minValue, mid - step, step, unit); // feature is < mid
}

export function parseIssueHref(href) {
  const path = (href || '').replace(/[#?].*$/, '');
  const [_, owner, repo, type, index] = /([^/]+)\/([^/]+)\/(issues|pulls)\/([0-9]+)/.exec(path) || [];
  return {owner, repo, type, index};
}

// parse a URL, either relative '/path' or absolute 'https://localhost/path'
export function parseUrl(str) {
  return new URL(str, str.startsWith('http') ? undefined : window.location.origin);
}

// return current locale chosen by user
function getCurrentLocale() {
  return document.documentElement.lang;
}

// given a month (0-11), returns it in the documents language
export function translateMonth(month) {
  return new Date(Date.UTC(2022, month, 12)).toLocaleString(getCurrentLocale(), {month: 'short', timeZone: 'UTC'});
}

// given a weekday (0-6, Sunday to Saturday), returns it in the documents language
export function translateDay(day) {
  return new Date(Date.UTC(2022, 7, day)).toLocaleString(getCurrentLocale(), {weekday: 'short', timeZone: 'UTC'});
}

// convert a Blob to a DataURI
export function blobToDataURI(blob) {
  return new Promise((resolve, reject) => {
    try {
      const reader = new FileReader();
      reader.addEventListener('load', (e) => {
        resolve(e.target.result);
      });
      reader.addEventListener('error', () => {
        reject(new Error('FileReader failed'));
      });
      reader.readAsDataURL(blob);
    } catch (err) {
      reject(err);
    }
  });
}

// convert image Blob to another mime-type format.
export function convertImage(blob, mime) {
  return new Promise(async (resolve, reject) => {
    try {
      const img = new Image();
      const canvas = document.createElement('canvas');
      img.addEventListener('load', () => {
        try {
          canvas.width = img.naturalWidth;
          canvas.height = img.naturalHeight;
          const context = canvas.getContext('2d');
          context.drawImage(img, 0, 0);
          canvas.toBlob((blob) => {
            if (!(blob instanceof Blob)) return reject(new Error('imageBlobToPng failed'));
            resolve(blob);
          }, mime);
        } catch (err) {
          reject(err);
        }
      });
      img.addEventListener('error', () => {
        reject(new Error('imageBlobToPng failed'));
      });
      img.src = await blobToDataURI(blob);
    } catch (err) {
      reject(err);
    }
  });
}

export function toAbsoluteUrl(url) {
  if (url.startsWith('http://') || url.startsWith('https://')) {
    return url;
  }
  if (url.startsWith('//')) {
    return `${window.location.protocol}${url}`; // it's also a somewhat absolute URL (with the current scheme)
  }
  if (url && !url.startsWith('/')) {
    throw new Error('unsupported url, it should either start with / or http(s)://');
  }
  return `${window.location.origin}${url}`;
}

// determine if light or dark text color should be used on a given background color
// NOTE: see models/issue_label.go for similar implementation
export function useLightTextOnBackground(backgroundColor) {
  if (backgroundColor[0] === '#') {
    backgroundColor = backgroundColor.substring(1);
  }
  // Perceived brightness from: https://www.w3.org/TR/AERT/#color-contrast
  // In the future WCAG 3 APCA may be a better solution.
  const r = parseInt(backgroundColor.substring(0, 2), 16);
  const g = parseInt(backgroundColor.substring(2, 4), 16);
  const b = parseInt(backgroundColor.substring(4, 6), 16);
  const brightness = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return brightness < 0.35;
}
