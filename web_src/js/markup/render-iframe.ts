import {generateElemId, queryElemChildren} from '../utils/dom.ts';
import {isDarkTheme} from '../utils.ts';

export async function loadRenderIframeContent(iframe: HTMLIFrameElement) {
  const iframeSrcUrl = iframe.getAttribute('data-src');
  if (!iframe.id) iframe.id = generateElemId('gitea-iframe-');

  window.addEventListener('message', (e) => {
    if (e.data && e.data.giteaIframeCmd === 'resize' && e.data.giteaIframeId === iframe.id) {
      iframe.style.height = `${e.data.giteaIframeHeight}px`;
    }
  });

  let urlParams = '';
  urlParams += `&gitea-is-dark-theme=${isDarkTheme()}`;
  urlParams += `&gitea-iframe-id=${iframe.id}`;
  iframe.src = iframeSrcUrl + (iframeSrcUrl.includes('?') ? '&' : '?') + urlParams.substring(1);
}

export function initMarkupRenderIframe(el: HTMLElement) {
  queryElemChildren(el, 'iframe.external-render-iframe', loadRenderIframeContent);
}
