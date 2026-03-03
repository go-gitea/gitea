import {generateElemId, queryElemChildren} from '../utils/dom.ts';
import {isDarkTheme} from '../utils.ts';

async function loadRenderIframeContent(iframe: HTMLIFrameElement) {
  const iframeSrcUrl = iframe.getAttribute('data-src')!;
  if (!iframe.id) iframe.id = generateElemId('gitea-iframe-');

  const normalizeOpenLink = (openLink: unknown): string | null => {
    if (typeof openLink !== 'string' || openLink === '') return null;
    let url: URL;
    try {
      url = new URL(openLink, window.location.href);
    } catch {
      return null;
    }
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return null;
    return url.href;
  };

  window.addEventListener('message', (e) => {
    if (e.source !== iframe.contentWindow) return;
    const data = e.data;
    if (!data?.giteaIframeCmd || data?.giteaIframeId !== iframe.id) return;
    const cmd = data.giteaIframeCmd;
    if (cmd === 'resize') {
      // TODO: sometimes the reported iframeHeight is not the size we need, need to figure why. Example: openapi swagger.
      //  As a workaround, add some pixels here.
      iframe.style.height = `${data.iframeHeight + 2}px`;
    } else if (cmd === 'open-link') {
      const openLink = normalizeOpenLink(data.openLink);
      if (!openLink) return;
      if (data.anchorTarget === '_blank') {
        window.open(openLink, '_blank', 'noopener');
      } else {
        window.location.assign(openLink);
      }
    } else {
      throw new Error(`Unknown gitea iframe cmd: ${cmd}`);
    }
  });

  const u = new URL(iframeSrcUrl, window.location.origin);
  u.searchParams.set('gitea-is-dark-theme', String(isDarkTheme()));
  u.searchParams.set('gitea-iframe-id', iframe.id);
  iframe.src = u.href;
}

export function initMarkupRenderIframe(el: HTMLElement) {
  queryElemChildren(el, 'iframe.external-render-iframe', loadRenderIframeContent);
}
