export function trPrintf(s: string, ...args: any[]) {
  let curIdx = 0;
  return s.replace(/%%|%(?:\[([1-9]\d*)\])?([sd])/g, (match, indexed: string) => {
    if (match === '%%') return '%';
    const argIndex = indexed ? Number(indexed) - 1 : curIdx++;
    if (argIndex < 0 || argIndex >= args.length) return match;
    return String(args[argIndex]);
  });
}
