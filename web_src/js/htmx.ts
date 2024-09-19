import {showErrorToast} from './modules/toast.ts';
import 'idiomorph/dist/idiomorph-ext.js'; // https://github.com/bigskysoftware/idiomorph#htmx
import 'htmx.org/dist/ext/ws.js';
import type {HtmxResponseInfo} from 'htmx.org';

type HtmxEvent = Event & {detail: HtmxResponseInfo};

// https://htmx.org/reference/#config
window.htmx.config.requestClass = 'is-loading';
window.htmx.config.scrollIntoViewOnBoost = false;

// https://htmx.org/events/#htmx:sendError
document.body.addEventListener('htmx:sendError', (event: HtmxEvent) => {
  // TODO: add translations
  showErrorToast(`Network error when calling ${e.detail.requestConfig.path}`);
});

// https://htmx.org/events/#htmx:responseError
document.body.addEventListener('htmx:responseError', (event: HtmxEvent) => {
  // TODO: add translations
  showErrorToast(`Error ${e.detail.xhr.status} when calling ${e.detail.requestConfig.path}`);
});

// TODO: move websocket creation to SharedWorker by overriding htmx.createWebSocket

document.body.addEventListener('htmx:wsOpen', (e) => {
  const socket = e.detail.socketWrapper;
  socket.send(
    JSON.stringify({action: 'subscribe', data: {url: window.location.href}})
  );
});
