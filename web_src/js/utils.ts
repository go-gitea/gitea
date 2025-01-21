import {decode, encode} from 'uint8-to-base64';
import type {IssuePageInfo, IssuePathInfo, RepoOwnerPathInfo} from './types.ts';

// transform /path/to/file.ext to file.ext
export function basename(path: string): string {
  const lastSlashIndex = path.lastIndexOf('/');
  return lastSlashIndex < 0 ? path : path.substring(lastSlashIndex + 1);
}

// transform /path/to/file.ext to .ext
export function extname(path: string): string {
  const lastSlashIndex = path.lastIndexOf('/');
  const lastPointIndex = path.lastIndexOf('.');
  if (lastSlashIndex > lastPointIndex) return '';
  return lastPointIndex < 0 ? '' : path.substring(lastPointIndex);
}

// test whether a variable is an object
export function isObject(obj: any): boolean {
  return Object.prototype.toString.call(obj) === '[object Object]';
}

// returns whether a dark theme is enabled
export function isDarkTheme(): boolean {
  const style = window.getComputedStyle(document.documentElement);
  return style.getPropertyValue('--is-dark-theme').trim().toLowerCase() === 'true';
}

// strip <tags> from a string
export function stripTags(text: string): string {
  return text.replace(/<[^>]*>?/g, '');
}

export function parseIssueHref(href: string): IssuePathInfo {
  // FIXME: it should use pathname and trim the appSubUrl ahead
  const path = (href || '').replace(/[#?].*$/, '');
  const [_, ownerName, repoName, pathType, indexString] = /([^/]+)\/([^/]+)\/(issues|pulls)\/([0-9]+)/.exec(path) || [];
  return {ownerName, repoName, pathType, indexString};
}

export function parseRepoOwnerPathInfo(pathname: string): RepoOwnerPathInfo {
  const appSubUrl = window.config.appSubUrl;
  if (appSubUrl && pathname.startsWith(appSubUrl)) pathname = pathname.substring(appSubUrl.length);
  const [_, ownerName, repoName] = /([^/]+)\/([^/]+)/.exec(pathname) || [];
  return {ownerName, repoName};
}

export function parseIssuePageInfo(): IssuePageInfo {
  const el = document.querySelector('#issue-page-info');
  return {
    issueNumber: parseInt(el?.getAttribute('data-issue-index')),
    issueDependencySearchType: el?.getAttribute('data-issue-dependency-search-type') || '',
    repoId: parseInt(el?.getAttribute('data-issue-repo-id')),
    repoLink: el?.getAttribute('data-issue-repo-link') || '',
  };
}

// parse a URL, either relative '/path' or absolute 'https://localhost/path'
export function parseUrl(str: string): URL {
  return new URL(str, str.startsWith('http') ? undefined : window.location.origin);
}

// return current locale chosen by user
export function getCurrentLocale(): string {
  return document.documentElement.lang;
}

// given a month (0-11), returns it in the documents language
export function translateMonth(month: number) {
  return new Date(Date.UTC(2022, month, 12)).toLocaleString(getCurrentLocale(), {month: 'short', timeZone: 'UTC'});
}

// given a weekday (0-6, Sunday to Saturday), returns it in the documents language
export function translateDay(day: number) {
  return new Date(Date.UTC(2022, 7, day)).toLocaleString(getCurrentLocale(), {weekday: 'short', timeZone: 'UTC'});
}

// convert a Blob to a DataURI
export function blobToDataURI(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    try {
      const reader = new FileReader();
      reader.addEventListener('load', (e) => {
        resolve(e.target.result as string);
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
export function convertImage(blob: Blob, mime: string): Promise<Blob> {
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

export function toAbsoluteUrl(url: string): string {
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

// Encode an Uint8Array into a URLEncoded base64 string.
export function encodeURLEncodedBase64(uint8Array: Uint8Array): string {
  return encode(uint8Array)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

// Decode a URLEncoded base64 to an Uint8Array.
export function decodeURLEncodedBase64(base64url: string): Uint8Array {
  return decode(base64url
    .replace(/_/g, '/')
    .replace(/-/g, '+'));
}

const domParser = new DOMParser();
const xmlSerializer = new XMLSerializer();

export function parseDom(text: string, contentType: DOMParserSupportedType): Document {
  return domParser.parseFromString(text, contentType);
}

export function serializeXml(node: Element | Node): string {
  return xmlSerializer.serializeToString(node);
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function isImageFile({name, type}: {name: string, type?: string}): boolean {
  return /\.(avif|jpe?g|png|gif|webp|svg|heic)$/i.test(name || '') || type?.startsWith('image/');
}

export function isVideoFile({name, type}: {name: string, type?: string}): boolean {
  return /\.(mpe?g|mp4|mkv|webm)$/i.test(name || '') || type?.startsWith('video/');
}
