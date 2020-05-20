// This sets up the URL prefix used in webpack's chunk loading.
// This file must be imported before any lazy-loading is being attempted.
const {StaticUrlPrefix} = window.config;

if (StaticUrlPrefix) {
  __webpack_public_path__ = StaticUrlPrefix.endsWith('/') ? StaticUrlPrefix : `${StaticUrlPrefix}/`;
} else if (document.currentScript && document.currentScript.src) {
  const url = new URL(document.currentScript.src);
  __webpack_public_path__ = url.pathname.replace(/\/[^/]*?\/[^/]*?$/, '/');
} else {
  // compat: IE11
  const script = document.querySelector('script[src*="/index.js"]');
  __webpack_public_path__ = script.getAttribute('src').replace(/\/[^/]*?\/[^/]*?$/, '/');
}
