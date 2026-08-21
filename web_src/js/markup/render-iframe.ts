import {generateElemId} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {isDarkTheme} from '../utils.ts';

function safeRenderIframeLink(link: any): string | null {
  try {
    const url = new URL(link, window.location.href);
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

  // There are 3 kinds of external render modes:
  // * external frontend render:
  //   * parent page creates iframe, iframe navigates to render page
  //   * render generates frame page with external-render-helper (injected), external-render-frontend and file content (hidden textarea)
  //   * frame page executes external-render-frontend JS code to finds a frontend plugin to render
  // * external backend render (HTML)
  //   * parent page creates iframe, iframe navigates to render page
  //   * render executes command to generate rendered HTML content with external-render-helper (injected)
  //   * frame page displays the rendered content
  // * external backend render (non-HTML, e.g.: PDF, image)
  //   * parent page creates iframe, iframe navigates to render page
  //   * render executes command to generate rendered content
  //   * response header is automatically detected from rendered content

  // It must use "src" here, because the frame content should not inherit parent's CSP.
  // Otherwise, "srcdoc" makes the frame content inherit the parent's CSP,
  // then some renders like "asciicast (asciinema)" which require "unsafe-eval" won't work.
  //
  // When using "src", Chrome can report false-alarm error like:
  // * Unsafe attempt to load URL http://localhost/owner/repo/render/branch/main/file from frame with URL http://localhost/owner/repo/render/branch/main/file. Domains, protocols and ports must match.
  // (only for the first time that the developer opens the browser console)
  // Such error log can also appear even if you access the link "http://.../owner/repo/render/branch/main/file" directly.
  // Everything just works, it is just a false-alarm caused by Chrome's Developer Tools, so such error log can be ignored.
  //
  // Another reason for why "src" is a must: if the render outputs non-HTML contents like PDF or image,
  // Only "src" can correctly load and display the rendered content, "srcdoc" won't work.
  iframe.src = u.href;
}
