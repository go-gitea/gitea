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
  const {runModeIsProd} = window.config ?? {};

  // The browser will log all `err` that pass this handler to the console, but some cases like
  // ResizeObserver [1] or Browser errors [2], [3] don't raise actual errors but only error events
  // which don't log to the console by default. We log these errors to the console, and during
  // development, they will additionaly show as error message.
  // [1] https://developer.mozilla.org/en-US/docs/Web/API/ResizeObserver#observation_errors
  // [2] https://github.com/mozilla-mobile/firefox-ios/issues/10817
  // [3] https://github.com/go-gitea/gitea/issues/20240
  if (!err && message) {
    console.error(new Error(message));
    if (runModeIsProd) return; // don't show error events in production
  }

  // If the error stack trace does not include the base URL of our script assets, it likely came
  // from a browser extension or inline script. Do not show such errors in production.
  if (!err?.stack?.includes(assetBaseUrl) && runModeIsProd) return;

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
