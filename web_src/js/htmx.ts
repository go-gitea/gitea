import {showErrorToast} from './modules/toast.ts';

// https://github.com/bigskysoftware/idiomorph#htmx
import 'idiomorph/dist/idiomorph-ext.js';

// https://htmx.org/reference/#config
window.htmx.config.requestClass = 'is-loading';
window.htmx.config.scrollIntoViewOnBoost = false;

// https://htmx.org/events/#htmx:sendError
document.body.addEventListener('htmx:sendError', (event) => {
  // TODO: add translations
  showErrorToast(`Network error when calling ${event.detail.requestConfig.path}`);
});

// https://htmx.org/events/#htmx:responseError
document.body.addEventListener('htmx:responseError', (event) => {
  // TODO: add translations
  showErrorToast(`Error ${event.detail.xhr.status} when calling ${event.detail.requestConfig.path}`);
});
