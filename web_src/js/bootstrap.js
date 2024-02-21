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
function processWindowErrorEvent(e) {
  const err = e.error ?? e.reason;
  const jsDir = `${window.location.origin}${__webpack_public_path__}js`;
  if (!err.stack?.includes(jsDir)) return; // error likely from browser extension

  let message;
  if (e.type === 'unhandledrejection') {
    message = `JavaScript promise rejection: ${err.message}.`;
  } else {
    message = `JavaScript error: ${e.message} (${e.filename} @ ${e.lineno}:${e.colno}).`;
  }
  showGlobalErrorMessage(`${message} Open browser console to see more details.`);
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
