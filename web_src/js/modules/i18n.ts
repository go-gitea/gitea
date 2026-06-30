import {getCurrentLocale} from '../utils.ts';

/** frontend `Locale.TrN`: pick the `_1` or `_n` form for `count` and interpolate `%d` */
export function trN(count: number, form1: string, formN: string, {lang = getCurrentLocale()}: {lang?: string} = {}): string {
  const form = new Intl.PluralRules(lang).select(count) === 'one' ? form1 : formN;
  return form.replace('%d', String(count));
}
