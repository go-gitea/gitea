declare module '*.svg' {
  const value: string;
  export default value;
}

declare module '*.css' {
  const value: string;
  export default value;
}

declare let __webpack_public_path__: string;

declare module 'htmx.org/dist/htmx.esm.js' {
  const value = await import('htmx.org');
  export default value;
}

declare module 'uint8-to-base64' {
  export function encode(arrayBuffer: ArrayBuffer): string;
  export function decode(base64str: string): ArrayBuffer;
}

declare module 'swagger-ui-dist/swagger-ui-es-bundle.js' {
  const value = await import('swagger-ui-dist');
  export default value.SwaggerUIBundle;
}

interface JQuery {
  api: any, // fomantic
  areYouSure: any, // jquery.are-you-sure
  dimmer: any, // fomantic
  dropdown: any; // fomantic
  modal: any; // fomantic
  tab: any; // fomantic
  transition: any, // fomantic
}

interface JQueryStatic {
  api: any, // fomantic
}

interface Element {
  _tippy: import('tippy.js').Instance;
}

type Writable<T> = { -readonly [K in keyof T]: T[K] };

interface Window {
  config: import('./web_src/js/types.ts').Config;
  $: typeof import('@types/jquery'),
  jQuery: typeof import('@types/jquery'),
  htmx: Omit<typeof import('htmx.org/dist/htmx.esm.js').default, 'config'> & {
    config?: Writable<typeof import('htmx.org').default.config>,
  },
  ui?: any,
  _globalHandlerErrors: Array<ErrorEvent & PromiseRejectionEvent> & {
    _inited: boolean,
    push: (e: ErrorEvent & PromiseRejectionEvent) => void | number,
  },
  __webpack_public_path__: string;
}
