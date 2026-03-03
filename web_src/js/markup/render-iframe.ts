import {generateElemId, queryElemChildren} from '../utils/dom.ts';
import {isDarkTheme} from '../utils.ts';

// arguments can be any type & any value, they are from "message" event's data
export function safeLinkHref(link: any): string | null {
  try {
    const url = new URL(`${link}`, window.location.href);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
      console.error(`Unsupported link protocol: ${link}`);
      return null;
    }
    return url.href;
  } catch (e) {
    console.error(`Failed to parse link: ${link}, error: ${e}`);
    return null;
  }
}

// arguments can be any type & any value, they are from "message" event's data
export function navigateToIframeLink(unsafeLink: any, target: any) {
  const linkHref = safeLinkHref(unsafeLink);
  if (linkHref === null) return;
  if (target === '_blank') {
    window.open(linkHref, '_blank', 'noopener,noreferrer');
    return;
  }
  // treat all other targets including ("_top", "_self", etc) as same tab navigation
  window.location.assign(linkHref);
}

async function loadRenderIframeContent(iframe: HTMLIFrameElement) {
  const iframeSrcUrl = iframe.getAttribute('data-src')!;
  if (!iframe.id) iframe.id = generateElemId('gitea-iframe-');

  window.addEventListener('message', (e) => {
    if (e.source !== iframe.contentWindow) return;
    if (!e.data?.giteaIframeCmd || e.data?.giteaIframeId !== iframe.id) return;
    const cmd = e.data.giteaIframeCmd;
    if (cmd === 'resize') {
      // TODO: sometimes the reported iframeHeight is not the size we need, need to figure why. Example: openapi swagger.
      //  As a workaround, add some pixels here.
      iframe.style.height = `${e.data.iframeHeight + 2}px`;
    } else if (cmd === 'open-link') {
      navigateToIframeLink(e.data.openLink, e.data.anchorTarget);
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
