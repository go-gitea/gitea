/* This sets up webpack's chunk loading to load resources from the same
  directory where it loaded index.js from. This file must be imported
  before any lazy-loading is being attempted. */

if (document.currentScript && document.currentScript.src) {
  const url = new URL(document.currentScript.src);
  __webpack_public_path__ = `${url.pathname.replace(/\/[^/]*$/, '')}/`;
} else {
  // compat: IE11
  const script = document.querySelector('script[src*="/index.js"]');
  __webpack_public_path__ = `${script.getAttribute('src').replace(/\/[^/]*$/, '')}/`;
}
