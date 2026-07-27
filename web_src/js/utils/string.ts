export function cutString(s: string, sep: string): [string, string, boolean] {
  const index = s.indexOf(sep);
  if (index === -1) return [s, '', false];
  return [s.substring(0, index), s.substring(index + sep.length), true];
}

export function trPrintf(s: string, ...args: any[]) {
  // TODO: refactor legacy ".replace('%[1]d')" and ".replace('%s')" calls to this function
  let curIdx = 0;
  return s.replace(/%%|%(?:\[([1-9]\d*)\])?([sd])/g, (match, indexed: string) => {
    if (match === '%%') return '%';
    const argIndex = indexed ? Number(indexed) - 1 : curIdx++;
    if (argIndex < 0 || argIndex >= args.length) return match;
    return String(args[argIndex]);
  });
}
