export function pathEscapeSegments(s) {
  return s.split('/').map(encodeURIComponent).join('/');
}
