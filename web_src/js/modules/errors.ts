// keep this file lightweight, it's imported into IIFE chunk in bootstrap
import {html} from '../utils/html.ts';
import octiconCheck from '../../../public/assets/img/svg/octicon-check.svg';
import octiconCopy from '../../../public/assets/img/svg/octicon-copy.svg';
import type {Intent} from '../types.ts';

export function showGlobalErrorMessage(msg: string, msgType: Intent = 'error', stack?: string) {
  const msgContainer = document.querySelector('.page-content') ?? document.body;
  if (!msgContainer) {
    alert(`${msgType}: ${msg}`);
    return;
  }
  const msgCompact = msg.replace(/\W/g, '').trim(); // compact the message to a data attribute to avoid too many duplicated messages
  let msgDiv = msgContainer.querySelector<HTMLDivElement>(`.js-global-error[data-global-error-msg-compact="${msgCompact}"]`);
  if (!msgDiv) {
    const el = document.createElement('div');
    el.innerHTML = html`
      <div class="ui container js-global-error tw-my-[--page-spacing]">
        <div class="ui ${msgType} message tw-flex tw-justify-center tw-items-center">
          <span class="js-global-error-msg"></span><span class="js-global-error-count"></span>
          <button type="button" class="js-global-error-copy interact-bg tw-text-inherit tw-p-2 tw-rounded tw-ml-1" aria-label="${window.config.i18n.copy}"></button>
          <pre class="js-global-error-stack tw-hidden"></pre>
        </div>
      </div>
    `;
    msgDiv = el.firstElementChild as HTMLDivElement;
    const copyBtn = msgDiv.querySelector<HTMLButtonElement>('.js-global-error-copy')!;
    copyBtn.innerHTML = octiconCopy;
    copyBtn.addEventListener('click', async () => {
      const msgText = msgDiv!.querySelector('.js-global-error-msg')!.textContent;
      const stackText = msgDiv!.querySelector('.js-global-error-stack')!.textContent;
      try {
        await navigator.clipboard?.writeText([msgText, stackText].filter(Boolean).join('\n'));
        copyBtn.innerHTML = octiconCheck;
        copyBtn.classList.replace('tw-text-inherit', 'tw-text-green');
        setTimeout(() => {
          copyBtn.innerHTML = octiconCopy;
          copyBtn.classList.replace('tw-text-green', 'tw-text-inherit');
        }, 1500);
      } catch {} // swallow clipboard failures so they don't trigger the global error handler
    });
  }
  const msgCount = Number(msgDiv.getAttribute('data-global-error-msg-count')) + 1;
  msgDiv.setAttribute('data-global-error-msg-compact', msgCompact);
  msgDiv.setAttribute('data-global-error-msg-count', String(msgCount));
  msgDiv.querySelector('.js-global-error-msg')!.textContent = msg;
  msgDiv.querySelector('.js-global-error-count')!.textContent = msgCount > 1 ? ` (${msgCount})` : '';
  if (stack) msgDiv.querySelector('.js-global-error-stack')!.textContent = stack;
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
