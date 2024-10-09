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
