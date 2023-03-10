import {createApp} from 'vue';

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

export function initVueApp(el, opts = {}) {
  if (typeof el === 'string') {
    el = document.querySelector(el);
  }
  if (!el) return null;

  return createApp(
    {delimiters: vueDelimiters, ...opts}
  ).mount(el);
}
