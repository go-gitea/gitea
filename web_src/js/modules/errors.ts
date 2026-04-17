// keep this file lightweight, it's imported into IIFE chunk in bootstrap
import {html} from '../utils/html.ts';
import type {Intent} from '../types.ts';

export function showGlobalErrorMessage(msg: string, msgType: Intent = 'error', details?: string) {
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
  const msgEl = msgDiv.querySelector('.ui.message')!;
  const text = msg + (msgCount > 1 ? ` (${msgCount})` : '');
  if (details) {
    if (!msgEl.querySelector('details')) {
      msgEl.classList.add('tw-cursor-pointer');
      msgEl.innerHTML = html`<details><summary></summary><pre class="tw-w-fit tw-mx-auto tw-mt-2 tw-mb-0 tw-whitespace-pre-wrap tw-text-left tw-cursor-text"><code class="tw-bg-transparent"></code></pre></details>`;
      const detailsEl = msgEl.querySelector('details')!;
      msgEl.addEventListener('click', (e) => {
        if (!(e.target as HTMLElement).closest('summary, pre')) detailsEl.open = !detailsEl.open;
      });
    }
    msgEl.querySelector('summary')!.textContent = text;
    msgEl.querySelector('pre code')!.textContent = details;
  } else {
    msgEl.textContent = text;
  }
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

  const renderedType = type === 'unhandledrejection' ? 'promise rejection' : type;
  let msg = err?.message ?? message;
  if (!err?.stack && lineno) msg += ` (${filename} @ ${lineno}:${colno})`;
  showGlobalErrorMessage(`JavaScript ${renderedType}: ${msg}`, 'error', err?.stack);
}
