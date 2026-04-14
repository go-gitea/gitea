// External render JS must be a IIFE module to run as early as possible to set up the environment for the content page.
// Avoid unnecessary dependency.
// Do NOT introduce global pollution, because the content page should be fully controlled by the external render.

/* To manually test:

[markup.in-iframe]
ENABLED = true
FILE_EXTENSIONS = .in-iframe
RENDER_CONTENT_MODE = iframe
RENDER_COMMAND = `echo '<div style="width: 100%; height: 2000px; border: 10px solid red; box-sizing: border-box;"><a href="/">a link</a> <a target="_blank" href="//gitea.com">external link</a></div>'`

;RENDER_COMMAND = cat /path/to/file.pdf
;RENDER_CONTENT_SANDBOX = disabled

*/

// Check whether the user-provided color value is a valid CSS color format to avoid CSS injection.
// Don't extract this function to a common module, because this file is an IIFE module for external render
// and should not have any dependency to avoid potential conflicts.
function isValidCssColor(s: string | null): boolean {
  if (!s) return false;
  // it should only be in format "#hex" or "rgb(...)", because it comes from a computed style's color value
  const reHex = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/;
  const reRgb = /^rgb\([^{}'";:]+\)$/;
  return reHex.test(s) || reRgb.test(s);
}

const url = new URL(window.location.href);

const isDarkTheme = url.searchParams.get('gitea-is-dark-theme') === 'true';
if (isDarkTheme) {
  document.documentElement.setAttribute('data-gitea-theme-dark', String(isDarkTheme));
}

const backgroundColor = url.searchParams.get('gitea-iframe-bgcolor');
if (isValidCssColor(backgroundColor)) {
  // create a style element to set background color, then it can be overridden by the content page's own style if needed
  const style = document.createElement('style');
  style.textContent = `
:root {
  --gitea-iframe-bgcolor: ${backgroundColor};
}
body { background: ${backgroundColor}; }
`;
  document.head.append(style);
}

const iframeId = url.searchParams.get('gitea-iframe-id');
if (iframeId) {
  // iframe is in different origin, so we need to use postMessage to communicate
  const postIframeMsg = (cmd: string, data: Record<string, any> = {}) => {
    window.parent.postMessage({giteaIframeCmd: cmd, giteaIframeId: iframeId, ...data}, '*');
  };

  const updateIframeHeight = () => {
    if (!document.body) return; // the body might not be available when this function is called
    // Use scrollHeight to get the full content height, even when CSS sets html/body to height:100%
    // (which would make getBoundingClientRect return the viewport height instead of content height).
    const height = Math.max(document.documentElement.scrollHeight, document.body.scrollHeight);
    postIframeMsg('resize', {iframeHeight: height});
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

if (window.testModules) {
  window.testModules.externalRenderHelper = {isValidCssColor};
}
