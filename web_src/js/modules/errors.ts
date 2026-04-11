// keep this file lightweight, it's imported into IIFE chunk in bootstrap
import {html} from '../utils/html.ts';
import type {Intent} from '../types.ts';

export function showGlobalErrorMessage(msg: string, msgType: Intent = 'error') {
  const msgContainer = document.querySelector('.page-content') ?? document.body;
  if (!msgContainer) {
    alert(`${msgType}: ${msg}`);
    return;
  }
  const msgCompact = msg.replace(/\W/g, '').trim(); // compact the message to a data attribute to avoid too many duplicated messages
  let msgDiv = msgContainer.querySelector<HTMLDivElement>(`.js-global-error[data-global-error-msg-compact="${msgCompact}"]`);
  if (!msgDiv) {
    const el = document.createElement('div');
    el.innerHTML = html`<div class="ui container js-global-error tw-my-[--page-spacing]"><div class="ui ${msgType} message tw-text-center tw-whitespace-pre-line"></div></div>`;
    msgDiv = el.childNodes[0] as HTMLDivElement;
  }
  // merge duplicated messages into "the message (count)" format
  const msgCount = Number(msgDiv.getAttribute(`data-global-error-msg-count`)) + 1;
  msgDiv.setAttribute(`data-global-error-msg-compact`, msgCompact);
  msgDiv.setAttribute(`data-global-error-msg-count`, msgCount.toString());
  msgDiv.querySelector('.ui.message')!.textContent = msg + (msgCount > 1 ? ` (${msgCount})` : '');
  msgContainer.prepend(msgDiv);
}

// Detect whether an error originated from Gitea's own scripts, not from
// browser extensions or other external scripts.
const extensionRe = /(chrome|moz|safari(-web)?)-extension:\/\//;
export function isGiteaError(filename: string, stack: string): boolean {
  if (extensionRe.test(filename) || extensionRe.test(stack)) return false;
  const assetBaseUrl = new URL(`${window.config.assetUrlPrefix}/`, window.location.origin).href;
  if (filename && !filename.startsWith(assetBaseUrl) && !filename.startsWith(window.location.origin)) return false;
  if (stack && !stack.includes(assetBaseUrl)) return false;
  return true;
}

export function processWindowErrorEvent({error, reason, message, type, filename, lineno, colno}: ErrorEvent & PromiseRejectionEvent) {
  const err = error ?? reason;
  // `error` and `reason` are not guaranteed to be errors. If the value is falsy, it is likely a
  // non-critical event from the browser. We log them but don't show them to users. Examples:
  // - https://developer.mozilla.org/en-US/docs/Web/API/ResizeObserver#observation_errors
  // - https://github.com/mozilla-mobile/firefox-ios/issues/10817
  // - https://github.com/go-gitea/gitea/issues/20240
  if (!err) {
    if (message) console.error(new Error(message));
    if (window.config.runModeIsProd) return;
  }

  // Filter out errors from browser extensions or other non-Gitea scripts.
  if (!isGiteaError(filename ?? '', err?.stack ?? '')) return;

  let msg = err?.message ?? message;
  if (lineno) msg += ` (${filename} @ ${lineno}:${colno})`;
  const dot = msg.endsWith('.') ? '' : '.';
  const renderedType = type === 'unhandledrejection' ? 'promise rejection' : type;
  showGlobalErrorMessage(`JavaScript ${renderedType}: ${msg}${dot} Open browser console to see more details.`);
}
