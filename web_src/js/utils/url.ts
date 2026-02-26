// Matches URLs, excluding characters that are never valid unencoded in URLs per RFC 3986
export const urlRawRegex = /\bhttps?:\/\/[^\s<>\[\]]+/gi;

export function cleanUrl(url: string): string {
  // Strip trailing punctuation that's likely not part of the URL
  url = url.replace(/[.,;:'"]+$/, '');
  // Strip trailing closing parens only if unbalanced (not part of the URL like Wikipedia links)
  while (url.endsWith(')') && (url.match(/\(/g) || []).length < (url.match(/\)/g) || []).length) {
    url = url.slice(0, -1);
  }
  return url;
}

export function findUrlAt(doc: string, pos: number): string | null {
  for (const match of doc.matchAll(urlRawRegex)) {
    const url = cleanUrl(match[0]);
    if (pos >= match.index && pos <= match.index + url.length) {
      return url;
    }
  }
  return null;
}

export function pathEscapeSegments(s: string): string {
  return s.split('/').map(encodeURIComponent).join('/');
}

/** Convert an absolute or relative URL to an absolute URL with the current origin. It only
 *  processes absolute HTTP/HTTPS URLs or relative URLs like '/xxx' or '//host/xxx'. */
export function toOriginUrl(urlStr: string) {
  try {
    if (urlStr.startsWith('http://') || urlStr.startsWith('https://') || urlStr.startsWith('/')) {
      const {origin, protocol, hostname, port} = window.location;
      const url = new URL(urlStr, origin);
      url.protocol = protocol;
      url.hostname = hostname;
      url.port = port || (protocol === 'https:' ? '443' : '80');
      return url.toString();
    }
  } catch {}
  return urlStr;
}
