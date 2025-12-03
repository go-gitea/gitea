import {generateElemId, queryElemChildren} from '../utils/dom.ts';
import {isDarkTheme} from '../utils.ts';

export async function loadRenderIframeContent(iframe: HTMLIFrameElement) {
  const iframeSrcUrl = iframe.getAttribute('data-src')!;
  if (!iframe.id) iframe.id = generateElemId('gitea-iframe-');

  window.addEventListener('message', (e) => {
    if (!e.data?.giteaIframeCmd || e.data?.giteaIframeId !== iframe.id) return;
    const cmd = e.data.giteaIframeCmd;
    if (cmd === 'resize') {
      iframe.style.height = `${e.data.iframeHeight}px`;
    } else if (cmd === 'open-link') {
      if (e.data.anchorTarget === '_blank') {
        window.open(e.data.openLink, '_blank');
      } else {
        window.location.href = e.data.openLink;
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
