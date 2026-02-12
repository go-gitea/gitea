declare module '@techknowlogick/license-checker-webpack-plugin' {
  const plugin: any;
  export = plugin;
}

declare module 'eslint-plugin-no-use-extend-native' {
  import type {Eslint} from 'eslint';
  const plugin: Eslint.Plugin;
  export = plugin;
}

declare module 'eslint-plugin-array-func' {
  import type {Eslint} from 'eslint';
  const plugin: Eslint.Plugin;
  export = plugin;
}

declare module 'eslint-plugin-github' {
  import type {Eslint} from 'eslint';
  const plugin: Eslint.Plugin;
  export = plugin;
}

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
  // Here we declare all exports from vue files so `tsc` or `tsgo` can work for
  // non-vue files. To lint .vue files, `vue-tsc` must be used.
  export function initDashboardRepoList(): void;
  export function initRepositoryActionView(): void;
}

declare module 'htmx.org/dist/htmx.esm.js' {
  const value = await import('htmx.org');
  export default value;
}

declare module 'swagger-ui-dist/swagger-ui-es-bundle.js' {
  const value = await import('swagger-ui-dist');
  export default value.SwaggerUIBundle;
}

declare module 'asciinema-player' {
  interface AsciinemaPlayer {
    create(src: string, element: HTMLElement, options?: Record<string, unknown>): void;
  }
  const exports: AsciinemaPlayer;
  export = exports;
}

declare module '@citation-js/core' {
  export class Cite {
    constructor(data: string);
    format(format: string, options?: Record<string, any>): string;
  }
  export const plugins: {
    config: {
      get(name: string): any;
    };
  };
}

declare module '@citation-js/plugin-software-formats' {}
declare module '@citation-js/plugin-bibtex' {}
declare module '@citation-js/plugin-csl' {}

declare module 'vue-bar-graph' {
  import type {DefineComponent} from 'vue';

  interface BarGraphPoint {
    value: number;
    label: string;
  }

  export const VueBarGraph: DefineComponent<{
    points?: Array<BarGraphPoint>;
    barColor?: string;
    textColor?: string;
    textAltColor?: string;
    height?: number;
    labelHeight?: number;
  }>;
}

declare module '@mcaptcha/vanilla-glue' {
  export let INPUT_NAME: string;
  export default class Widget {
    constructor(options: {
      siteKey: {
        instanceUrl: URL;
        key: string;
      };
    });
  }
}
