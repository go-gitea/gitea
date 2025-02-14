import {showErrorToast} from './modules/toast.ts';
import 'idiomorph/dist/idiomorph-ext.js'; // https://github.com/bigskysoftware/idiomorph#htmx
import type {HtmxResponseInfo} from 'htmx.org';

type HtmxEvent = Event & {detail: HtmxResponseInfo};

// https://htmx.org/reference/#config
window.htmx.config.requestClass = 'is-loading';
window.htmx.config.scrollIntoViewOnBoost = false;

// https://htmx.org/events/#htmx:sendError
document.body.addEventListener('htmx:sendError', (event: Partial<HtmxEvent>) => {
  // TODO: add translations
  showErrorToast(`Network error when calling ${event.detail.requestConfig.path}`);
});

// https://htmx.org/events/#htmx:responseError
document.body.addEventListener('htmx:responseError', (event: Partial<HtmxEvent>) => {
  // TODO: add translations
  showErrorToast(`Error ${event.detail.xhr.status} when calling ${event.detail.requestConfig.path}`);
});
