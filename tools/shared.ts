import vuePlugin from '@vitejs/plugin-vue';
import {stringPlugin} from 'vite-string-plugin';
import type {Plugin} from 'vite';

// custom elements, vue must render these as-is instead of resolving them as components
const webComponents = new Set([
  // our own, in web_src/js/webcomponents
  'overflow-menu',
  'relative-time',
  // from dependencies
  'markdown-toolbar',
  'text-expander',
]);

export const vueDefines = {
  __VUE_OPTIONS_API__: false,
  __VUE_PROD_DEVTOOLS__: false,
  __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: false,
};

export const sharedResolve = {
  alias: {
    vue: 'vue/dist/vue.runtime.esm-bundler.js', // https://github.com/vuejs/core/issues/14416
  },
};

export function sharedPlugins(): Plugin[] {
  return [
    stringPlugin(),
    vuePlugin({
      template: {
        compilerOptions: {
          isCustomElement: (tag) => webComponents.has(tag),
        },
      },
    }),
  ];
}
