import {showTemporaryTooltip} from '../modules/tippy.js';
import {toAbsoluteUrl} from '../utils.js';
import {clippie} from 'clippie';

const {copy_success, copy_error} = window.config.i18n;

// For all DOM elements with [data-clipboard-target] or [data-clipboard-text],
// this copy-to-clipboard will work for them
export function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', (e) => {
    let target = e.target;
    // in case <button data-clipboard-text><svg></button>, so we just search
    // up to 3 levels for performance
    for (let i = 0; i < 3 && target; i++) {
      let txt = target.getAttribute('data-clipboard-text');
      if (txt && target.getAttribute('data-clipboard-text-type') === 'url') {
        txt = toAbsoluteUrl(txt);
      }
      const text = txt || document.querySelector(target.getAttribute('data-clipboard-target'))?.value;

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
