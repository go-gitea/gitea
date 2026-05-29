import {getCurrentLocale} from '../utils.ts';

/** frontend counterpart to the backend `Locale.TrN`: pick the singular (`_1`) or plural (`_n`) form for `count` and interpolate `%d` */
export function trN(count: number, form1: string, formN: string): string {
  const locale = getCurrentLocale();
  const form = new Intl.PluralRules(locale).select(count) === 'one' ? form1 : formN;
  return form.replace('%d', count.toLocaleString(locale));
}
