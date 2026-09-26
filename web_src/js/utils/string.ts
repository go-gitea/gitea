export function cutString(s: string, sep: string): [string, string, boolean] {
  const index = s.indexOf(sep);
  if (index === -1) return [s, '', false];
  return [s.substring(0, index), s.substring(index + sep.length), true];
}
