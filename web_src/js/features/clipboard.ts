import {showTemporaryTooltip} from '../modules/tippy.ts';
import {toAbsoluteUrl} from '../utils.ts';
import {clippie} from 'clippie';
import type {DOMEvent} from '../utils/dom.ts';

const {copy_success, copy_error} = window.config.i18n;

// Enable clipboard copy from HTML attributes. These properties are supported:
// - data-clipboard-text: Direct text to copy
// - data-clipboard-target: Holds a selector for a <input> or <textarea> whose content is copied
// - data-clipboard-text-type: When set to 'url' will convert relative to absolute urls
export function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', async (e: DOMEvent<MouseEvent>) => {
    const target = e.target.closest('[data-clipboard-text], [data-clipboard-target]');
    if (!target) return;

    e.preventDefault();

    let text = target.getAttribute('data-clipboard-text');
    if (!text) {
      text = document.querySelector<HTMLInputElement>(target.getAttribute('data-clipboard-target'))?.value;
    }

    if (text && target.getAttribute('data-clipboard-text-type') === 'url') {
      text = toAbsoluteUrl(text);
    }

    if (text) {
      const success = await clippie(text);
      showTemporaryTooltip(target, success ? copy_success : copy_error);
    }
  });
}
