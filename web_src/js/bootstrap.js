import {joinPaths} from './utils.js';

// DO NOT IMPORT window.config HERE!
// to make sure the error handler always works, we should never import `window.config`, because some user's custom template breaks it.

// This sets up the URL prefix used in webpack's chunk loading.
// This file must be imported before any lazy-loading is being attempted.
__webpack_public_path__ = joinPaths(window?.config?.assetUrlPrefix ?? '/', '/');

export function showGlobalErrorMessage(msg) {
  const pageContent = document.querySelector('.page-content');
  if (!pageContent) return;
  const el = document.createElement('div');
  el.innerHTML = `<div class="ui container negative message center aligned js-global-error" style="white-space: pre-line;"></div>`;
  el.childNodes[0].textContent = msg;
  pageContent.prepend(el.childNodes[0]);
}

/**
 * @param {ErrorEvent} e
 */
function processWindowErrorEvent(e) {
  if (window.config.initCount > 1) {
    // the page content has been loaded many times, the HTML/JS are totally broken, don't need to show error message
    return;
  }
  if (!e.error && e.lineno === 0 && e.colno === 0 && e.filename === '' && window.navigator.userAgent.includes('FxiOS/')) {
    // At the moment, Firefox (iOS) (10x) has an engine bug. See https://github.com/go-gitea/gitea/issues/20240
    // If a script inserts a newly created (and content changed) element into DOM, there will be a nonsense error event reporting: Script error: line 0, col 0.
    return; // ignore such nonsense error event
  }

  showGlobalErrorMessage(`JavaScript error: ${e.message} (${e.filename} @ ${e.lineno}:${e.colno}). Open browser console to see more details.`);
}

function initGlobalErrorHandler() {
  if (!window.config) {
    showGlobalErrorMessage(`Gitea JavaScript code couldn't run correctly, please check your custom templates`);
  }
  if (window.config.initCount > 1) {
    // when a sub-templates triggers an 500 error, its parent template has been partially rendered,
    // then the 500 page will be rendered after that partially rendered page, which will cause the initCount > 1
    // in this case, the page is totally broken, so do not do any further error handling
    console.error('initGlobalErrorHandler: Gitea global config system has already been initialized, there must be something else wrong');
    return;
  }
  // we added an event handler for window error at the very beginning of <script> of page head
  // the handler calls `_globalHandlerErrors.push` (array method) to record all errors occur before this init
  // then in this init, we can collect all error events and show them
  for (const e of window._globalHandlerErrors || []) {
    processWindowErrorEvent(e);
  }
  // then, change _globalHandlerErrors to an object with push method, to process further error events directly
  window._globalHandlerErrors = {'push': (e) => processWindowErrorEvent(e)};
}

initGlobalErrorHandler();
