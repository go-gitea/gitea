// DO NOT IMPORT window.config HERE!
// to make sure the error handler always works, we should never import `window.config`, because
// some user's custom template breaks it.
import {showGlobalErrorMessage, processWindowErrorEvent} from './modules/errors.ts';

// A module should not be imported twice, otherwise there will be bugs when a module has its internal states.
// A real example is "generateElemId" in "utils/dom.ts", if it is imported twice in different module scopes,
// It will generate duplicate IDs (ps: don't try to use "random" to fix, it is just a real example to show the importance of "do not import a module twice")
if (!window._globalHandlerErrors?._inited) {
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
