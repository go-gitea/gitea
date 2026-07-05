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
}

declare module 'idiomorph' {
  interface IdiomorphCallbacks {
    beforeNodeAdded?(node: Node): boolean | void;
    afterNodeAdded?(node: Node): void;
    beforeNodeMorphed?(oldNode: Node, newNode: Node): boolean | void;
    afterNodeMorphed?(oldNode: Node, newNode: Node): void;
    beforeNodeRemoved?(node: Node): boolean | void;
    afterNodeRemoved?(node: Node): void;
    beforeAttributeUpdated?(attributeName: string, node: Node, mutationType: 'update' | 'remove'): boolean | void;
  }

  interface IdiomorphOptions {
    morphStyle?: 'innerHTML' | 'outerHTML';
    ignoreActive?: boolean;
    ignoreActiveValue?: boolean;
    restoreFocus?: boolean;
    callbacks?: IdiomorphCallbacks;
  }

  interface Idiomorph {
    morph(existing: Node | string, replacement: Node | string, options?: IdiomorphOptions): void;
  }
  export const Idiomorph: Idiomorph;
}

declare module 'swagger-ui-dist/swagger-ui-es-bundle.js' {
  const value = await import('swagger-ui-dist');
  export default value.SwaggerUIBundle;
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
