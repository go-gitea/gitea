declare module '*.svg' {
  const value: string;
  export default value;
}

declare module '*.txt' {
  const value: string;
  export default value;
}

declare module '*.vue' {
  import type {DefineComponent} from 'vue';
  const component: DefineComponent<unknown, unknown, any>;
  export default component;
}

declare module 'idiomorph' {
  interface Idiomorph {
    morph(existing: Node | string, replacement: Node | string, options?: {morphStyle: 'innerHTML' | 'outerHTML'}): void;
  }
  export const Idiomorph: Idiomorph;
}

declare module 'swagger-ui-dist/swagger-ui-es-bundle.js' {
  const value = await import('swagger-ui-dist');
  export default value.SwaggerUIBundle;
}
