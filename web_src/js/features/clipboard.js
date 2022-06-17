import $ from 'jquery';

const {copy_success, copy_error} = window.config.i18n;

function onSuccess(btn) {
  btn.setAttribute('data-variation', 'inverted tiny');
  $(btn).popup('destroy');
  const oldContent = btn.getAttribute('data-content');
  btn.setAttribute('data-content', copy_success);
  $(btn).popup('show');
  btn.setAttribute('data-content', oldContent || '');
}
function onError(btn) {
  btn.setAttribute('data-variation', 'inverted tiny');
  const oldContent = btn.getAttribute('data-content');
  $(btn).popup('destroy');
  btn.setAttribute('data-content', copy_error);
  $(btn).popup('show');
  btn.setAttribute('data-content', oldContent || '');
}


// Fallback to use if navigator.clipboard doesn't exist. Achieved via creating
// a temporary textarea element, selecting the text, and using document.execCommand
function fallbackCopyToClipboard(text) {
  if (!document.execCommand) return false;

  const tempTextArea = document.createElement('textarea');
  tempTextArea.value = text;

  // avoid scrolling
  tempTextArea.style.top = 0;
  tempTextArea.style.left = 0;
  tempTextArea.style.position = 'fixed';

  document.body.appendChild(tempTextArea);

  tempTextArea.select();

  // if unsecure (not https), there is no navigator.clipboard, but we can still
  // use document.execCommand to copy to clipboard
  const success = document.execCommand('copy');

  document.body.removeChild(tempTextArea);

  return success;
}

// For all DOM elements with [data-clipboard-target] or [data-clipboard-text],
// this copy-to-clipboard will work for them
export default function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', (e) => {
    let target = e.target;
    // in case <button data-clipboard-text><svg></button>, so we just search
    // up to 3 levels for performance
    for (let i = 0; i < 3 && target; i++) {
      const text = target.getAttribute('data-clipboard-text') || document.querySelector(target.getAttribute('data-clipboard-target'))?.value;

      if (text) {
        e.preventDefault();

        (async() => {
          try {
            await navigator.clipboard.writeText(text);
            onSuccess(target);
          } catch {
            if (fallbackCopyToClipboard(text)) {
              onSuccess(target);
            } else {
              onError(target);
            }
          }
        })();

        break;
      }
      target = target.parentElement;
    }
  });
}
