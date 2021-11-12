// For all DOM elements with [data-clipboard-target] or [data-clipboard-text], this copy-to-clipboard will work for them

// TODO: replace these with toast-style notifications
function onSuccess(btn) {
  if (!btn.dataset.content) return;
  $(btn).popup('destroy');
  const oldContent = btn.dataset.content;
  btn.dataset.content = btn.dataset.success;
  $(btn).popup('show');
  btn.dataset.content = oldContent;
}
function onError(btn) {
  if (!btn.dataset.content) return;
  const oldContent = btn.dataset.content;
  $(btn).popup('destroy');
  btn.dataset.content = btn.dataset.error;
  $(btn).popup('show');
  btn.dataset.content = oldContent;
}

/**
 * Fallback to use if navigator.clipboard doesn't exist.
 * Achieved via creating a temporary textarea element, selecting the text, and using document.execCommand.
 */
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

  // if unsecure (not https), there is no navigator.clipboard, but we can still use document.execCommand to copy to clipboard
  const success = document.execCommand('copy');

  document.body.removeChild(tempTextArea);

  return success;
}

export default function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', (e) => {
    let target = e.target;
    // in case <button data-clipboard-text><svg></button>, so we just search up to 3 levels for performance.
    for (let i = 0; i < 3 && target; i++) {
      let text;
      if (target.dataset.clipboardText) {
        text = target.dataset.clipboardText;
      } else if (target.dataset.clipboardTarget) {
        text = document.querySelector(target.dataset.clipboardTarget)?.value;
      }
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
