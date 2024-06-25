export function pathEscapeSegments(s) {
  return s.split('/').map(encodeURIComponent).join('/');
}

function stripSlash(url) {
  return url.endsWith('/') ? url.slice(0, -1) : url;
}

export function isUrl(url) {
  try {
    return stripSlash((new URL(url).href)).trim() === stripSlash(url).trim();
  } catch {
    return false;
  }
}
