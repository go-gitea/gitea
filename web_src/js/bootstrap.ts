// DO NOT IMPORT window.config HERE!
// to make sure the error handler always works, we should never import `window.config`, because
// some user's custom template breaks it.
import type {Intent} from './types.ts';

// This sets up the URL prefix used in webpack's chunk loading.
// This file must be imported before any lazy-loading is being attempted.
__webpack_public_path__ = `${window.config?.assetUrlPrefix ?? '/assets'}/`;

function shouldIgnoreError(err: Error) {
  const ignorePatterns = [
    '/assets/js/monaco.', // https://github.com/go-gitea/gitea/issues/30861 , https://github.com/microsoft/monaco-editor/issues/4496
  ];
  for (const pattern of ignorePatterns) {
    if (err.stack?.includes(pattern)) return true;
  }
  return false;
}

export function showGlobalErrorMessage(msg: string, msgType: Intent = 'error') {
  const msgContainer = document.querySelector('.page-content') ?? document.body;
  const msgCompact = msg.replace(/\W/g, '').trim(); // compact the message to a data attribute to avoid too many duplicated messages
  let msgDiv = msgContainer.querySelector<HTMLDivElement>(`.js-global-error[data-global-error-msg-compact="${msgCompact}"]`);
  if (!msgDiv) {
    const el = document.createElement('div');
    el.innerHTML = `<div class="ui container js-global-error tw-my-[--page-spacing]"><div class="ui ${msgType} message tw-text-center tw-whitespace-pre-line"></div></div>`;
    msgDiv = el.childNodes[0] as HTMLDivElement;
  }
  // merge duplicated messages into "the message (count)" format
  const msgCount = Number(msgDiv.getAttribute(`data-global-error-msg-count`)) + 1;
  msgDiv.setAttribute(`data-global-error-msg-compact`, msgCompact);
  msgDiv.setAttribute(`data-global-error-msg-count`, msgCount.toString());
  msgDiv.querySelector('.ui.message').textContent = msg + (msgCount > 1 ? ` (${msgCount})` : '');
  msgContainer.prepend(msgDiv);
}

function processWindowErrorEvent({error, reason, message, type, filename, lineno, colno}: ErrorEvent & PromiseRejectionEvent) {
  const err = error ?? reason;
  const assetBaseUrl = String(new URL(__webpack_public_path__, window.location.origin));
  const {runModeIsProd} = window.config ?? {};

  // `error` and `reason` are not guaranteed to be errors. If the value is falsy, it is likely a
  // non-critical event from the browser. We log them but don't show them to users. Examples:
  // - https://developer.mozilla.org/en-US/docs/Web/API/ResizeObserver#observation_errors
  // - https://github.com/mozilla-mobile/firefox-ios/issues/10817
  // - https://github.com/go-gitea/gitea/issues/20240
  if (!err) {
    if (message) console.error(new Error(message));
    if (runModeIsProd) return;
  }

  if (err instanceof Error) {
    // If the error stack trace does not include the base URL of our script assets, it likely came
    // from a browser extension or inline script. Do not show such errors in production.
    if (!err.stack?.includes(assetBaseUrl) && runModeIsProd) return;
    // Ignore some known errors that are unable to fix
    if (shouldIgnoreError(err)) return;
  }

  let msg = err?.message ?? message;
  if (lineno) msg += ` (${filename} @ ${lineno}:${colno})`;
  const dot = msg.endsWith('.') ? '' : '.';
  const renderedType = type === 'unhandledrejection' ? 'promise rejection' : type;
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
  // we added an event handler for window error at the very beginning of <script> of page head the
  // handler calls `_globalHandlerErrors.push` (array method) to record all errors occur before
  // this init then in this init, we can collect all error events and show them.
  for (const e of window._globalHandlerErrors || []) {
    processWindowErrorEvent(e);
  }
  // then, change _globalHandlerErrors to an object with push method, to process further error
  // events directly
  // @ts-expect-error -- this should be refactored to not use a fake array
  window._globalHandlerErrors = {_inited: true, push: (e: ErrorEvent & PromiseRejectionEvent) => processWindowErrorEvent(e)};
}

initGlobalErrorHandler();
