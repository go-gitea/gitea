export function fomanticQuery(s: string | Element | NodeListOf<Element>): ReturnType<typeof $> {
  // intentionally make it only work for query selector, it isn't used for creating HTML elements (for safety)
  return typeof s === 'string' ? $(document).find(s) : $(s);
}
