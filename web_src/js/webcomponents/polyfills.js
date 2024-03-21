try {
  // some browsers like PaleMoon don't have full support for Intl.NumberFormat, so do the minimum polyfill to support "relative-time-element"
  // https://repo.palemoon.org/MoonchildProductions/UXP/issues/2289
  new Intl.NumberFormat('en', {style: 'unit', unit: 'minute'}).format(1);
} catch {
  const intlNumberFormat = Intl.NumberFormat;
  Intl.NumberFormat = function(locales, options) {
    if (options.style === 'unit') {
      return {
        format(value) {
          return ` ${value} ${options.unit}`;
        }
      };
    }
    return intlNumberFormat(locales, options);
  };
}
