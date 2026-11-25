import type {FunctionalComponent} from 'vue';
import {getCurrentLocale} from '../utils.ts';

/** frontend `Locale.TrN`: pick the `_1` or `_n` form for `count` and interpolate `%d` */
export function trN(count: number, form1: string, formN: string, {lang = getCurrentLocale()}: {lang?: string} = {}): string {
  const text = new Intl.PluralRules(lang).select(count) === 'one' ? form1 : formN;
  return trString(text, String(count));
}

function trParts<T>(s: string, getArg: (argNum: number) => T | undefined): Array<string | T> {
  // the same behavior (almost) as backend TrString
  let curIdx = 0;
  return s.split(/(%%|%(?:\[[1-9]\d*\])?[sd])/).map((part, index) => {
    if (index % 2 === 0) return part;
    if (part === '%%') return '%';
    return getArg(part.length > 2 ? Number(part.slice(2, -2)) : ++curIdx) ?? part;
  });
}

/**
 * Fills variables in a translated string (like TrString in Golang)
 * Only `%s`, `%d` or one-based position like `%[1]s` are supported.
 */
export function trString(s: string, ...args: Array<string | number>) {
  return trParts(s, (argNum) => args[argNum - 1]).join('');
}

/** `trString` as a Vue component, slot N fills argument N, so arguments can be elements (like Tr in Golang) */
export const TrSlots: FunctionalComponent<{text: string}> = ({text}, {slots}) => trParts(text, (argNum) => slots[argNum]?.());
