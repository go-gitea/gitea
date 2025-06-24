declare module '*.svg' {
  const value: string;
  export default value;
}

declare module '*.css' {
  const value: string;
  export default value;
}

declare module '*.vue' {
  import type {DefineComponent} from 'vue';
  const component: DefineComponent<unknown, unknown, any>;
  export default component;
  // List of named exports from vue components, used to make `tsc` output clean.
  // To actually lint .vue files, `vue-tsc` is used because `tsc` can not parse them.
  export function initDashboardRepoList(): void;
  export function initRepositoryActionView(): void;
}

declare let __webpack_public_path__: string;

declare module 'htmx.org/dist/htmx.esm.js' {
  const value = await import('htmx.org');
  export default value;
}

declare module 'uint8-to-base64' {
  export function encode(arrayBuffer: Uint8Array): string;
  export function decode(base64str: string): Uint8Array;
}

declare module 'swagger-ui-dist/swagger-ui-es-bundle.js' {
  const value = await import('swagger-ui-dist');
  export default value.SwaggerUIBundle;
}

interface JQuery {
  areYouSure: any, // jquery.are-you-sure
  fomanticExt: any; // fomantic extension
  api: any, // fomantic
  dimmer: any, // fomantic
  dropdown: any; // fomantic
  modal: any; // fomantic
  tab: any; // fomantic
  transition: any, // fomantic
  search: any, // fomantic
}

interface JQueryStatic {
  api: any, // fomantic
}

interface Element {
  _tippy: import('tippy.js').Instance;
}

type Writable<T> = { -readonly [K in keyof T]: T[K] };

interface Window {
  __webpack_public_path__: string;
  config: import('./web_src/js/types.ts').Config;
  $: typeof import('@types/jquery'),
  jQuery: typeof import('@types/jquery'),
  htmx: Omit<typeof import('htmx.org/dist/htmx.esm.js').default, 'config'> & {
    config?: Writable<typeof import('htmx.org').default.config>,
    process?: (elt: Element | string) => void,
  },
  _globalHandlerErrors: Array<ErrorEvent & PromiseRejectionEvent> & {
    _inited: boolean,
    push: (e: ErrorEvent & PromiseRejectionEvent) => void | number,
  },
  codeEditors: any[], // export editor for customization

  // various captcha plugins
  grecaptcha: any,
  turnstile: any,
  hcaptcha: any,

  // do not add more properties here unless it is a must
}
