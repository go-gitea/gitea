// keep this file lightweight, it's imported into IIFE chunk in bootstrap
import {html} from '../utils/html.ts';
import type {Intent} from '../types.ts';

export function errorMessage(err: unknown): string {
  return (err as Error)?.message || String(err);
}

export function showGlobalErrorMessage(msg: string, msgType: Intent = 'error', details?: string) {
  const parentContainer = document.querySelector('.page-content') ?? document.body;
  if (!parentContainer) {
    alert(`${msgType}: ${msg}`);
    return;
  }
  // compact the message to a data attribute to avoid too many duplicated messages
  const msgCompact = `${msgType}-${msg.trim()}`.replace(/[^-\w\u{80}-\u{10FFFF}]+/gu, '');
  let msgContainer = parentContainer.querySelector<HTMLDivElement>(`.js-global-error[data-global-error-msg-compact="${msgCompact}"]`);
  if (!msgContainer) {
    const el = document.createElement('div');
    el.innerHTML = html`<div class="ui container js-global-error tw-my-[--page-spacing]"><details class="ui ${msgType} message"><summary></summary></details></div>`;
    msgContainer = el.childNodes[0] as HTMLDivElement;
  }

  // merge duplicated messages into "the message (count)" format
  const msgCount = Number(msgContainer.getAttribute(`data-global-error-msg-count`)) + 1;
  msgContainer.setAttribute(`data-global-error-msg-compact`, msgCompact);
  msgContainer.setAttribute(`data-global-error-msg-count`, msgCount.toString());

  const msgElem = msgContainer.querySelector('details')!;
  const msgSummary = msgElem.querySelector('summary')!;
  msgSummary.textContent = msg + (msgCount > 1 ? ` (${msgCount})` : '');
  if (details) {
    let msgDetailsPre = msgElem.querySelector('pre');
    if (!msgDetailsPre) msgDetailsPre = document.createElement('pre');
    msgDetailsPre.textContent = details;
    msgElem.append(msgDetailsPre);
  }
  parentContainer.prepend(msgContainer);
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

  const renderedType = type === 'unhandledrejection' ? 'promise rejection' : type;
  let msg = err?.message ?? message;
  if (!err?.stack && lineno) msg += ` (${filename} @ ${lineno}:${colno})`;
  const dot = msg.endsWith('.') ? '' : '.';
  showGlobalErrorMessage(`JavaScript ${renderedType}: ${msg}${dot} Open browser console to see more details.`, 'error', err?.stack);
}
