// This sets up webpack's chunk loading to load resources from the 'public'
// directory. This file must be imported before any lazy-loading is being attempted.

const url = new URL(document.currentScript.src);
__webpack_public_path__ = url.pathname.replace(/\/[^/]*?\/[^/]*?$/, '/');
