// DO NOT IMPORT window.config HERE!
// to make sure the error handler always works, we should never import `window.config`, because some user's custom template breaks it.

// This sets up the URL prefix used in webpack's chunk loading.
// This file must be imported before any lazy-loading is being attempted.
__webpack_public_path__ = `${window.config?.assetUrlPrefix ?? '/assets'}/`;

export function showGlobalErrorMessage(msg) {
  const pageContent = document.querySelector('.page-content');
  if (!pageContent) return;

  // compact the message to a data attribute to avoid too many duplicated messages
  const msgCompact = msg.replace(/\W/g, '').trim();
  let msgDiv = pageContent.querySelector(`.js-global-error[data-global-error-msg-compact="${msgCompact}"]`);
  if (!msgDiv) {
    const el = document.createElement('div');
    el.innerHTML = `<div class="ui container negative message center aligned js-global-error" style="white-space: pre-line;"></div>`;
    msgDiv = el.childNodes[0];
  }
  // merge duplicated messages into "the message (count)" format
  const msgCount = Number(msgDiv.getAttribute(`data-global-error-msg-count`)) + 1;
  msgDiv.setAttribute(`data-global-error-msg-compact`, msgCompact);
  msgDiv.setAttribute(`data-global-error-msg-count`, msgCount.toString());
  msgDiv.textContent = msg + (msgCount > 1 ? ` (${msgCount})` : '');
  pageContent.prepend(msgDiv);
}

/**
 * @param {ErrorEvent} e
 */
function processWindowErrorEvent({error, reason, message, type, filename, lineno, colno}) {
  const err = error ?? reason;
  const assetBaseUrl = String(new URL(__webpack_public_path__, window.location.origin));

  // Normally the browser will log the error to the console, but in some cases like "ResizeObserver
  // loop completed with undelivered notifications" in Firefox, e.error is undefined, resulting in
  // nothing being logged by the browser, so we do it instead.
  if (!err && message) console.error(new Error(message));

  // If the error stack trace does not include the base URL of our scripts, it is likely from a
  // browser extension or inline script. Do not show these in production builds.
  if (!err?.stack?.includes(assetBaseUrl) && window.config?.runModeIsProd) return;

  // At the moment, Firefox (iOS) (10x) has an engine bug. If a script inserts a newly created (and
  // content changed) element into DOM, there will be a nonsense error event reporting: Script
  // error: line 0, col 0, ignore such nonsense error event.
  // See https://github.com/go-gitea/gitea/issues/20240
  if (!err && lineno === 0 && colno === 0 && filename === '' && window.navigator.userAgent.includes('FxiOS/')) return;

  const renderedType = type === 'unhandledrejection' ? 'promise rejection' : type;
  let msg = err?.message ?? message;
  if (lineno) msg += `(${filename} @ ${lineno}:${colno})`;
  const dot = msg.endsWith('.') ? '' : '.';
  showGlobalErrorMessage(`JavaScript ${renderedType}: ${msg}${dot} Open browser console to see more details.`);
}

function initGlobalErrorHandler() {
  if (window._globalHandlerErrors?._inited) {
    showGlobalErrorMessage(`The global error handler has been initialized, do not initialize it again`);
    return;
  }
  if (!window.config) {
    showGlobalErrorMessage(`Gitea JavaScript code couldn't run correctly, please check your custom templates`);
  }
  // we added an event handler for window error at the very beginning of <script> of page head
  // the handler calls `_globalHandlerErrors.push` (array method) to record all errors occur before this init
  // then in this init, we can collect all error events and show them
  for (const e of window._globalHandlerErrors || []) {
    processWindowErrorEvent(e);
  }
  // then, change _globalHandlerErrors to an object with push method, to process further error events directly
  window._globalHandlerErrors = {_inited: true, push: (e) => processWindowErrorEvent(e)};
}

initGlobalErrorHandler();
