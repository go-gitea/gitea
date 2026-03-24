// DO NOT IMPORT window.config HERE!
// to make sure the error handler always works, we should never import `window.config`, because
// some user's custom template breaks it.
import {showGlobalErrorMessage} from './modules/message.ts';

// This file must be imported before any lazy-loading is being attempted.

export function shouldIgnoreError(err: Error) {
  const ignorePatterns: Array<RegExp> = [
    // https://github.com/go-gitea/gitea/issues/30861
    // https://github.com/microsoft/monaco-editor/issues/4496
    // https://github.com/microsoft/monaco-editor/issues/4679
    /\/assets\/js\/.*(monaco|editor\.(api|worker))/,
  ];
  for (const pattern of ignorePatterns) {
    if (pattern.test(err.stack ?? '')) return true;
  }
  return false;
}

function processWindowErrorEvent({error, reason, message, type, filename, lineno, colno}: ErrorEvent & PromiseRejectionEvent) {
  const err = error ?? reason;
  const assetBaseUrl = String(new URL(`${window.config?.assetUrlPrefix ?? '/assets'}/`, window.location.origin));
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
    // A module should not be imported twice, otherwise there will be bugs when a module has its internal states.
    // A real example is "generateElemId" in "utils/dom.ts", if it is imported twice in different module scopes,
    // It will generate duplicate IDs (ps: don't try to use "random" to fix, it is just a real example to show the importance of "do not import a module twice")
    return;
  }
  if (!window.config) {
    showGlobalErrorMessage(`Gitea JavaScript code couldn't run correctly, please check your custom templates`);
  }
  // we added an event handler for window error at the very beginning of <script> of page head the
  // handler calls `_globalHandlerErrors.push` (array method) to record all errors occur before
  // this init then in this init, we can collect all error events and show them.
  for (const e of (window._globalHandlerErrors as Iterable<ErrorEvent & PromiseRejectionEvent>) || []) {
    processWindowErrorEvent(e);
  }
  // then, change _globalHandlerErrors to an object with push method, to process further error
  // events directly
  window._globalHandlerErrors = {_inited: true, push: (e: ErrorEvent & PromiseRejectionEvent) => processWindowErrorEvent(e)} as any;
}

initGlobalErrorHandler();
