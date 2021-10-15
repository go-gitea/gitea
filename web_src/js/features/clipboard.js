// For all DOM elements with [data-clipboard-target] or [data-clipboard-text], this copy-to-clipboard will work for them

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

export default function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', async (e) => {
    const target = e.target;
    let text;
    if (target.dataset.clipboardText) {
      text = target.dataset.clipboardText;
    } else if (target.dataset.clipboardTarget) {
      text = document.querySelector(target.dataset.clipboardTarget)?.value;
    }
    if (!text) return;

    try {
      await navigator.clipboard.writeText(text);
      onSuccess(target);
    } catch {
      onError(target);
    }
  });
}
