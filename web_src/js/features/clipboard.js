import {showTemporaryTooltip} from '../modules/tippy.js';
import {toAbsoluteUrl} from '../utils.js';
import {clippie} from 'clippie';

const {copy_success, copy_error} = window.config.i18n;

// Enable clipboard copy from HTML attributes. These properties are supported:
// - data-clipboard-text: Direct text to copy, has highest precedence
// - data-clipboard-target: Holds a selector for a <input> or <textarea> whose content is copied
// - data-clipboard-text-type: When set to 'url' will convert relative to absolute urls
export function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', (e) => {
    let target = e.target;
    // In case <button data-clipboard-text><svg></button>, so we just search
    // up to 3 levels for performance
    for (let i = 0; i < 3 && target; i++) {
      let text = target.getAttribute('data-clipboard-text');

      if (!text && target.getAttribute('data-clipboard-target')) {
        text = document.querySelector(target.getAttribute('data-clipboard-target'))?.value;
      }

      if (text && target.getAttribute('data-clipboard-text-type') === 'url') {
        text = toAbsoluteUrl(text);
      }

      if (text) {
        e.preventDefault();

        (async() => {
          const success = await clippie(text);
          showTemporaryTooltip(target, success ? copy_success : copy_error);
        })();

        break;
      }

      target = target.parentElement;
    }
  });
}
