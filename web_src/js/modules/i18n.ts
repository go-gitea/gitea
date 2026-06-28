import {getCurrentLocale} from '../utils.ts';

/** frontend `Locale.TrN`: pick the `_1` or `_n` form for `count` and interpolate `%d` */
export function trN(count: number, form1: string, formN: string): string {
  let pluralRules: Intl.PluralRules;
  try {
    pluralRules = new Intl.PluralRules(getCurrentLocale());
  } catch {
    pluralRules = new Intl.PluralRules('en');
  }
  const form = pluralRules.select(count) === 'one' ? form1 : formN;
  return form.replace('%d', String(count));
}
