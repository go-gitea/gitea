const selector = '[data-clipboard-target], [data-clipboard-text]';

// TODO: replace these with toast-style notifications
function onSuccess(btn) {
  if (!btn.dataset.content) return;
  $(btn).popup('destroy');
  btn.dataset.content = btn.dataset.success;
  $(btn).popup('show');
  btn.dataset.content = btn.dataset.original;
}
function onError(btn) {
  if (!btn.dataset.content) return;
  $(btn).popup('destroy');
  btn.dataset.content = btn.dataset.error;
  $(btn).popup('show');
  btn.dataset.content = btn.dataset.original;
}

/** Use the document.execCommand to copy the value to clipboard */
function fallbackCopyViaSelect(elem) {
  elem.select();

  // if unsecure (not https), there is no navigator.clipboard, but we can still use document.execCommand to copy to clipboard
  // it's also fine if we don't test it exists because of the try statement
  return document.execCommand('copy');
}
/**
 * Fallback to use if navigator.clipboard doesn't exist.
 * Achieved via creating a temporary textarea element, selecting the text, and using document.execCommand.
 */
function fallbackCopyToClipboard(text) {
  const tempTextArea = document.createElement('textarea');
  tempTextArea.value = text;

  // avoid scrolling
  tempTextArea.style.top = 0;
  tempTextArea.style.left = 0;
  tempTextArea.style.position = 'fixed';

  document.body.appendChild(tempTextArea);

  const success = fallbackCopyViaSelect(tempTextArea);

  document.body.removeChild(tempTextArea);

  return success;
}

export default async function initClipboard() {
  for (const btn of document.querySelectorAll(selector) || []) {
    btn.addEventListener('click', async () => {
      let text;
      if (btn.dataset.clipboardText) {
        text = btn.dataset.clipboardText;
      } else if (btn.dataset.clipboardTarget) {
        text = document.querySelector(btn.dataset.clipboardTarget)?.value;
      }
      if (!text) return;

      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(text);
          onSuccess(btn);
        } else {
          const success = btn.dataset.clipboardTarget ? fallbackCopyViaSelect(document.querySelector(btn.dataset.clipboardTarget)) : fallbackCopyToClipboard(text);
          if (success) {
            onSuccess(btn);
          } else {
            onError(btn);
          }
        }
      } catch {
        onError(btn);
      }
    });
  }
}
