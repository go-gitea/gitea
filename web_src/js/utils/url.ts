export function pathEscapeSegments(s: string): string {
  return s.split('/').map(encodeURIComponent).join('/');
}

function stripSlash(url: string): string {
  return url.endsWith('/') ? url.slice(0, -1) : url;
}

export function isUrl(url: string): boolean {
  try {
    return stripSlash((new URL(url).href)).trim() === stripSlash(url).trim();
  } catch {
    return false;
  }
}

// Convert an absolute or relative URL to an absolute URL with the current origin. It only
// processes absolute HTTP/HTTPS URLs or relative URLs like '/xxx' or '//host/xxx'.
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
