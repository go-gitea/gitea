import {generateElemId} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {isDarkTheme} from '../utils.ts';
import {GET} from '../modules/fetch.ts';

function safeRenderIframeLink(link: any): string | null {
  try {
    const url = new URL(`${link}`, window.location.href);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
      console.error(`Unsupported link protocol: ${link}`);
      return null;
    }
    return url.href;
  } catch (e) {
    console.error(`Failed to parse link: ${link}, error: ${errorMessage(e)}`);
    return null;
  }
}

// This function is only designed for "open-link" command from iframe, is not suitable for other contexts.
// Because other link protocols are directly handled by the iframe, but not here.
// Arguments can be any type & any value, they are from "message" event's data which is not controlled by us.
export function navigateToIframeLink(unsafeLink: any, target: any) {
  const linkHref = safeRenderIframeLink(unsafeLink);
  if (linkHref === null) return;
  if (target === '_blank') {
    window.open(linkHref, '_blank', 'noopener,noreferrer');
    return;
  }
  // treat all other targets including ("_top", "_self", etc.) as same tab navigation
  window.location.assign(linkHref);
}

function getRealBackgroundColor(el: HTMLElement) {
  for (let n = el; n; n = n.parentElement!) {
    const style = window.getComputedStyle(n);
    const bgColor = style.backgroundColor;
    // 'rgba(0, 0, 0, 0)' is how most browsers represent transparent
    if (bgColor !== 'rgba(0, 0, 0, 0)' && bgColor !== 'transparent') {
      return bgColor;
    }
  }
  return '';
}

export async function initExternalRenderIframe(iframe: HTMLIFrameElement) {
  const iframeSrcUrl = iframe.getAttribute('data-src')!;
  if (!iframe.id) iframe.id = generateElemId('gitea-iframe-');

  window.addEventListener('message', (e) => {
    if (e.source !== iframe.contentWindow) return;
    if (!e.data?.giteaIframeCmd || e.data?.giteaIframeId !== iframe.id) return;
    const cmd = e.data.giteaIframeCmd;
    if (cmd === 'resize') {
      iframe.style.height = `${e.data.iframeHeight}px`;
    } else if (cmd === 'open-link') {
      navigateToIframeLink(e.data.openLink, e.data.anchorTarget);
    } else {
      throw new Error(`Unknown gitea iframe cmd: ${cmd}`);
    }
  });

  const u = new URL(iframeSrcUrl, window.location.origin);
  u.searchParams.set('gitea-is-dark-theme', String(isDarkTheme()));
  u.searchParams.set('gitea-iframe-id', iframe.id);
  u.searchParams.set('gitea-iframe-bgcolor', getRealBackgroundColor(iframe));

  // It must use "srcdoc" here, because our backend always sends CSP sandbox directive for the rendered content
  // (to protect from XSS risks), so we can't use "src" to load the content directly, otherwise there will be console errors like:
  // Unsafe attempt to load URL http://localhost:3000/test from frame with URL http://localhost:3000/test
  const resp = await GET(u.href);
  iframe.srcdoc = await resp.text();
}
