declare module '*.svg' {
  const value: string;
  export default value;
}

declare let __webpack_public_path__: string;

interface Window {
  config: import('./web_src/js/types.ts').Config;
  $: typeof import('@types/jquery'),
  jQuery: typeof import('@types/jquery'),
  htmx: typeof import('htmx.org'),
  _globalHandlerErrors: Array<ErrorEvent & PromiseRejectionEvent> & {
    _inited: boolean,
    push: (e: ErrorEvent & PromiseRejectionEvent) => void | number,
  },
}

declare module 'htmx.org/dist/htmx.esm.js' {
  const value = await import('htmx.org');
  export default value;
}
