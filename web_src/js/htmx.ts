import htmx from 'htmx.org';
import 'idiomorph/htmx';
import type {HtmxResponseInfo} from 'htmx.org';
import {showErrorToast} from './modules/toast.ts';

type HtmxEvent = Event & {detail: HtmxResponseInfo};

export function initHtmx() {
  window.htmx = htmx;

  // https://htmx.org/reference/#config
  htmx.config.requestClass = 'is-loading';
  htmx.config.scrollIntoViewOnBoost = false;

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
}
