/* To manually test:

[markup.in-iframe]
ENABLED = true
FILE_EXTENSIONS = .in-iframe
RENDER_CONTENT_MODE = iframe
RENDER_COMMAND = `echo '<div style="width: 100%; height: 2000px; border: 10px solid red; box-sizing: border-box;">content</div>'`

*/

function mainExternalRenderIframe() {
  const u = new URL(window.location.href);
  const fn = () => window.parent.postMessage({
    giteaIframeCmd: 'resize',
    giteaIframeId: u.searchParams.get('gitea-iframe-id'),
    giteaIframeHeight: document.documentElement.scrollHeight,
  }, '*');
  fn();
  window.addEventListener('DOMContentLoaded', fn);
  setInterval(fn, 1000);

  // make all absolute links open in new window (otherwise they would be blocked by all parents' frame-src)
  document.body.addEventListener('click', (e) => {
    const el = e.target as HTMLAnchorElement;
    if (el.nodeName !== 'A') return;
    const href = el.getAttribute('href');
    if (!href.startsWith('//') && !href.includes('://')) return;
    el.target = '_blank';
  }, true);
}

mainExternalRenderIframe();
