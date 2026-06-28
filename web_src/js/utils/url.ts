import {html, htmlRaw} from './html.ts';

export function urlQueryEscape(s: string) {
  // See "TestQueryEscape" in backend
  // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/encodeURIComponent#encoding_for_rfc3986
  return encodeURIComponent(s).replace(
    /[!'()*]/g,
    (c) => `%${c.charCodeAt(0).toString(16).toUpperCase()}`,
  ).replaceAll('%20', '+');
}

export function pathEscape(s: string): string {
  // See "TestPathEscape" in backend
  return encodeURIComponent(s).replace(
    /[!'()*]/g,
    (c) => `%${c.charCodeAt(0).toString(16).toUpperCase()}`,
  ).replaceAll(/%(\w\w)/g, (v) => {
    switch (v) {
      case '%24': return '$';
      case '%26': return '&';
      case '%2B': return '+';
      case '%3A': return ':';
      case '%3D': return '=';
      case '%40': return '@';
      default: return v;
    }
  });
}

export function pathEscapeSegments(s: string): string {
  // The same as backend's PathEscapeSegments
  return s.split('/').map(pathEscape).join('/');
}
