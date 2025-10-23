import {generateElemId, queryElemChildren} from '../utils/dom.ts';
import {isDarkTheme} from '../utils.ts';

export async function loadRenderIframeContent(iframe: HTMLIFrameElement) {
  const iframeSrcUrl = iframe.getAttribute('data-src');
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
  iframe.src = iframeSrcUrl + (iframeSrcUrl.includes('?') ? '&' : '?') + String(new URLSearchParams([
    ['gitea-is-dark-theme', String(isDarkTheme())],
    ['gitea-iframe-id', iframe.id],
  ]));
}

export function initMarkupRenderIframe(el: HTMLElement) {
  queryElemChildren(el, 'iframe.external-render-iframe', loadRenderIframeContent);
}
