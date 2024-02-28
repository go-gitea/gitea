import {showTemporaryTooltip} from '../modules/tippy.js';
import {toAbsoluteUrl} from '../utils.js';
import {clippie} from 'clippie';

const {copy_success, copy_error} = window.config.i18n;

// Enable clipboard copy from HTML attributes. These properties are supported:
// - data-clipboard-text: Direct text to copy
// - data-clipboard-target: Holds a selector for a <input> or <textarea> whose content is copied
// - data-clipboard-text-type: When set to 'url' will convert relative to absolute urls
export function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', async (e) => {
    const target = e.target.closest('[data-clipboard-text], [data-clipboard-target]');
    if (!target) return;

    e.preventDefault();

    let text;
    if (target.hasAttribute('data-clipboard-text')) {
      text = target.getAttribute('data-clipboard-text');
    } else {
      text = document.querySelector(target.getAttribute('data-clipboard-target'))?.value;
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
