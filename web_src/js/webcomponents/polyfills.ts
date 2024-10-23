try {
  // some browsers like PaleMoon don't have full support for Intl.NumberFormat, so do the minimum polyfill to support "relative-time-element"
  // https://repo.palemoon.org/MoonchildProductions/UXP/issues/2289
  new Intl.NumberFormat('en', {style: 'unit', unit: 'minute'}).format(1);
} catch {
  const intlNumberFormat = Intl.NumberFormat;
  // @ts-expect-error - polyfill is incomplete
  Intl.NumberFormat = function(locales: string | string[], options: Intl.NumberFormatOptions) {
    if (options.style === 'unit') {
      return {
        format(value: number | bigint | string) {
          return ` ${value} ${options.unit}`;
        },
      };
    }
    return intlNumberFormat(locales, options);
  };
}
