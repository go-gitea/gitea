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
        if(navigator.clipboard && window.isSecureContext){

          await navigator.clipboard.writeText(text);
          onSuccess(btn);
        }else{
          if(btn.dataset.clipboardTarget) {
            // if unsecure (not https), there is no navigator.clipboard, but we can still use document.execCommand to copy
            // it's also fine if we don't test it exists because of the try statement
            document.querySelector(btn.dataset.clipboardTarget).select();
            if(document.execCommand('copy')){
              onSuccess(btn);
            }else{
              onError(btn);
            }
          }else{
            onError(btn);
          }
        }
      } catch {
        onError(btn);
      }
    });
  }
}
