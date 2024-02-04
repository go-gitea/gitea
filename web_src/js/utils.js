import {encode, decode} from 'uint8-to-base64';

// transform /path/to/file.ext to file.ext
export function basename(path = '') {
  return path ? path.replace(/^.*\//, '') : '';
}

// transform /path/to/file.ext to .ext
export function extname(path = '') {
  const [_, ext] = /.+(\.[^.]+)$/.exec(path) || [];
  return ext || '';
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
export function getCurrentLocale() {
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

// Encode an ArrayBuffer into a URLEncoded base64 string.
export function encodeURLEncodedBase64(arrayBuffer) {
  return encode(arrayBuffer)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

// Decode a URLEncoded base64 to an ArrayBuffer string.
export function decodeURLEncodedBase64(base64url) {
  return decode(base64url
    .replace(/_/g, '/')
    .replace(/-/g, '+'));
}

const domParser = new DOMParser();
const xmlSerializer = new XMLSerializer();

export function parseDom(text, contentType) {
  return domParser.parseFromString(text, contentType);
}

export function serializeXml(node) {
  return xmlSerializer.serializeToString(node);
}
