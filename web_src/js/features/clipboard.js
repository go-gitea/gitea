export default async function initClipboard() {
  const els = document.querySelectorAll('.clipboard');
  if (!els || !els.length) return;

  const {default: ClipboardJS} = await import(/* webpackChunkName: "clipboard" */'clipboard');

  const clipboard = new ClipboardJS(els);
  clipboard.on('success', (e) => {
    e.clearSelection();

    $(`#${e.trigger.getAttribute('id')}`).popup('destroy');
    e.trigger.setAttribute('data-content', e.trigger.getAttribute('data-success'));
    $(`#${e.trigger.getAttribute('id')}`).popup('show');
    e.trigger.setAttribute('data-content', e.trigger.getAttribute('data-original'));
  });

  clipboard.on('error', (e) => {
    $(`#${e.trigger.getAttribute('id')}`).popup('destroy');
    e.trigger.setAttribute('data-content', e.trigger.getAttribute('data-error'));
    $(`#${e.trigger.getAttribute('id')}`).popup('show');
    e.trigger.setAttribute('data-content', e.trigger.getAttribute('data-original'));
  });
}
