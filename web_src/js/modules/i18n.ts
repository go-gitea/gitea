import {getCurrentLocale} from '../utils.ts';

/** frontend `Locale.TrN`: pick the `_1` or `_n` form for `count` and interpolate `%d` */
export function trN(count: number, form1: string, formN: string, {lang = getCurrentLocale()}: {lang?: string} = {}): string {
  const text = new Intl.PluralRules(lang).select(count) === 'one' ? form1 : formN;
  return trString(text, String(count));
}

export function trString(s: string, ...args: any[]) {
  // the same behavior (almost) as backend TrString
  let curIdx = 0;
  return s.replace(/%%|%(?:\[([1-9]\d*)\])?([sd])/g, (match, indexed: string) => {
    if (match === '%%') return '%';
    const argIndex = indexed ? Number(indexed) - 1 : curIdx++;
    if (argIndex < 0 || argIndex >= args.length) return match;
    return String(args[argIndex]);
  });
}
