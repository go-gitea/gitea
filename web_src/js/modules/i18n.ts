import type {Slots} from 'vue';
import {getCurrentLocale} from '../utils.ts';

/** frontend `Locale.TrN`: pick the `_1` or `_n` form for `count` and interpolate `%d` */
export function trN(count: number, form1: string, formN: string, {lang = getCurrentLocale()}: {lang?: string} = {}): string {
  const text = new Intl.PluralRules(lang).select(count) === 'one' ? form1 : formN;
  return trString(text, String(count));
}

/** Replace placeholders via `getArg` */
function trParts<T>(s: string, getArg: (argNum: number) => T | undefined): Array<string | T> {
  let curIdx = 0;
  return s.split(/(%%|%(?:\[[1-9]\d*\])?[sd])/).map((part, index) => {
    if (index % 2 === 0) return part;
    if (part === '%%') return '%';
    return getArg(part.length > 2 ? Number(part.slice(2, -2)) : ++curIdx) ?? part;
  });
}

/** Fill `%s`, `%d` and `%[1]s` placeholders, like backend TrString */
export function trString(s: string, ...args: Array<string | number>) {
  return trParts(s, (argNum) => args[argNum - 1]).join('');
}

/** Component form of trString, slot N fills placeholder N */
export function trSlots({text}: {text: string}, {slots}: {slots: Slots}) {
  return trParts(text, (argNum) => slots[argNum]?.());
}
