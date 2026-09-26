/** Matches URLs, excluding characters that are never valid unencoded in URLs per RFC 3986. */
export const urlRawRegex = () => /\bhttps?:\/\/[^\s<>[\]]+/gi; // JS regexp has internal states, so always use a new instance

/** Strip trailing punctuation that is likely not part of the URL. */
export function trimUrlPunctuation(url: string): string {
  url = url.replace(/[.,;:'"]+$/, '');
  // Strip trailing closing parens only if unbalanced (not part of the URL like Wikipedia links),
  // counted once as a URL can carry as many parens as the text it was found in is long
  let unbalanced = 0;
  for (const char of url) unbalanced += char === ')' ? 1 : char === '(' ? -1 : 0;
  let strip = 0;
  while (strip < unbalanced && url[url.length - 1 - strip] === ')') strip++;
  return strip ? url.slice(0, -strip) : url;
}

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
