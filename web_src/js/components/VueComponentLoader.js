import Vue from 'vue';
import {svgs} from '../svg.js';

export const vueDelimiters = ['${', '}'];

let vueEnvInited = false;
export function initVueEnv() {
  if (vueEnvInited) return;
  vueEnvInited = true;

  const isProd = window.config.IsProd;
  Vue.config.productionTip = false;
  Vue.config.devtools = !isProd;
}

let vueSvgInited = false;
export function initVueSvg() {
  if (vueSvgInited) return;
  vueSvgInited = true;

  // register svg icon vue components, e.g. <octicon-repo size="16"/>
  for (const [name, htmlString] of Object.entries(svgs)) {
    const template = htmlString
      .replace(/height="[0-9]+"/, 'v-bind:height="size"')
      .replace(/width="[0-9]+"/, 'v-bind:width="size"');

    Vue.component(name, {
      props: {
        size: {
          type: String,
          default: '16',
        },
      },
      template,
    });
  }
}

export function initVueApp(el, opts = {}) {
  if (typeof el === 'string') {
    el = document.querySelector(el);
  }
  if (!el) return null;

  return new Vue(Object.assign({
    el,
    delimiters: vueDelimiters,
  }, opts));
}
