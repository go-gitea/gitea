/* To manually test:

[markup.in-iframe]
ENABLED = true
FILE_EXTENSIONS = .in-iframe
RENDER_CONTENT_MODE = iframe
RENDER_COMMAND = `echo '<div style="width: 100%; height: 2000px; border: 10px solid red; box-sizing: border-box;"><a href="/">a link</a> <a target="_blank" href="//gitea.com">external link</a></div>'`

;RENDER_COMMAND = cat /path/to/file.pdf
;RENDER_CONTENT_SANDBOX = disabled

*/

function mainExternalRenderIframe() {
  const u = new URL(window.location.href);
  const iframeId = u.searchParams.get('gitea-iframe-id');

  // iframe is in different origin, so we need to use postMessage to communicate
  const postIframeMsg = (cmd: string, data: Record<string, any> = {}) => {
    window.parent.postMessage({giteaIframeCmd: cmd, giteaIframeId: iframeId, ...data}, '*');
  };

  const updateIframeHeight = () => {
    // Don't use integer heights from the DOM node.
    // Use getBoundingClientRect(), then ceil the height to avoid fractional pixels which causes incorrect scrollbars.
    const rect = document.documentElement.getBoundingClientRect();
    postIframeMsg('resize', {iframeHeight: Math.ceil(rect.height)});
    // As long as the parent page is responsible for the iframe height, the iframe itself doesn't need scrollbars.
    // This style should only be dynamically set here when our code can run.
    document.documentElement.style.overflowY = 'hidden';
  };
  const resizeObserver = new ResizeObserver(() => updateIframeHeight());
  resizeObserver.observe(window.document.documentElement);

  updateIframeHeight();
  window.addEventListener('DOMContentLoaded', updateIframeHeight);
  // the easiest way to handle dynamic content changes and easy to debug, can be fine-tuned in the future
  setInterval(updateIframeHeight, 1000);

  // no way to open an absolute link with CSP frame-src, it needs some tricks like "postMessage" (let parent window to handle) or "copy the link to clipboard" (let users manually paste it to open).
  // here we choose "postMessage" way for better user experience.
  const openIframeLink = (link: string, target: string | null) => postIframeMsg('open-link', {openLink: link, anchorTarget: target});
  document.addEventListener('click', (e) => {
    const el = e.target as HTMLAnchorElement;
    if (el.nodeName !== 'A') return;
    const href = el.getAttribute('href') ?? '';
    // safe links: "./any", "../any", "/any", "//host/any", "http://host/any", "https://host/any"
    if (href.startsWith('.') || href.startsWith('/') || href.startsWith('http://') || href.startsWith('https://')) {
      e.preventDefault();
      const forceTarget = (e.metaKey || e.ctrlKey) ? '_blank' : null;
      openIframeLink(href, forceTarget ?? el.getAttribute('target'));
    }
  });
}

mainExternalRenderIframe();
