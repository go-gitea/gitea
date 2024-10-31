import {showErrorToast} from './modules/toast.ts';
import 'idiomorph/dist/idiomorph-ext.js'; // https://github.com/bigskysoftware/idiomorph#htmx
import 'htmx-ext-ws';
import type {HtmxResponseInfo} from 'htmx.org';

type SocketWrapper = {
  send: (msg: string) => void;
};
type HtmxEvent = Event & {detail: HtmxResponseInfo & {socketWrapper: SocketWrapper}};

// https://htmx.org/reference/#config
window.htmx.config.requestClass = 'is-loading';
window.htmx.config.scrollIntoViewOnBoost = false;

// https://htmx.org/events/#htmx:sendError
document.body.addEventListener('htmx:sendError', (event: HtmxEvent) => {
  // TODO: add translations
  showErrorToast(`Network error when calling ${event.detail.requestConfig.path}`);
});

// https://htmx.org/events/#htmx:responseError
document.body.addEventListener('htmx:responseError', (event: HtmxEvent) => {
  // TODO: add translations
  showErrorToast(`Error ${event.detail.xhr.status} when calling ${event.detail.requestConfig.path}`);
});

// TODO: move websocket creation to SharedWorker by overriding htmx.createWebSocket

document.body.addEventListener('htmx:wsOpen', (event: HtmxEvent) => {
  const socket = event.detail.socketWrapper;
  socket.send(
    JSON.stringify({action: 'subscribe', data: {url: window.location.href}}),
  );
});
