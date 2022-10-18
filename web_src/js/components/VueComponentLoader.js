import {createApp} from 'vue';
import {svgs} from '../svg.js';

export const vueDelimiters = ['${', '}'];

let vueEnvInited = false;
export function initVueEnv() {
  if (vueEnvInited) return;
  vueEnvInited = true;

  // As far as I could tell, this is no longer possible.
  // But there seem not to be a guide what to do instead.
  // const isProd = window.config.runModeIsProd;
  // Vue.config.devtools = !isProd;
}

let vueSvgInited = false;
export function initVueSvg(app) {
  if (vueSvgInited) return;
  vueSvgInited = true;

  // register svg icon vue components, e.g. <octicon-repo size="16"/>
  for (const [name, htmlString] of Object.entries(svgs)) {
    const template = htmlString
      .replace(/height="[0-9]+"/, 'v-bind:height="size"')
      .replace(/width="[0-9]+"/, 'v-bind:width="size"');

    app.component(name, {
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

  return createApp(
    {delimiters: vueDelimiters, ...opts}
  ).mount(el);
}
