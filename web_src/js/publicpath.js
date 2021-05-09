// This sets up the URL prefix used in webpack's chunk loading.
// This file must be imported before any lazy-loading is being attempted.
const {AssetUrlPrefix} = window.config;

if (AssetUrlPrefix) {
  __webpack_public_path__ = AssetUrlPrefix.endsWith('/') ? AssetUrlPrefix : `${AssetUrlPrefix}/`;
} else {
  const url = new URL(document.currentScript.src);
  __webpack_public_path__ = url.pathname.replace(/\/[^/]*?\/[^/]*?$/, '/');
}
