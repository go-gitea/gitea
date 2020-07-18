export default async function initClipboard() {
  const els = document.querySelectorAll('.clipboard');
  if (!els || !els.length) return;

  const {default: ClipboardJS} = await import(/* webpackChunkName: "clipboard" */'clipboard');

  const clipboard = new ClipboardJS(els);
  clipboard.on('success', (e) => {
    e.clearSelection();
    $(e.trigger).popup('destroy');
    e.trigger.dataset.content = e.trigger.dataset.success;
    $(e.trigger).popup('show');
    e.trigger.dataset.content = e.trigger.dataset.original;
  });

  clipboard.on('error', (e) => {
    $(e.trigger).popup('destroy');
    e.trigger.dataset.content = e.trigger.dataset.error;
    $(e.trigger).popup('show');
    e.trigger.dataset.content = e.trigger.dataset.original;
  });
}
