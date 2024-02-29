// Convert an absolute or relative URL to an absolute URL with the current origin. It only
// processes absolute HTTP/HTTPS URLs or relative URLs like '/xxx' or '//host/xxx'.
// NOTE: Keep this function in sync with clone_script.tmpl
export function toOriginUrl(urlStr) {
  try {
    if (urlStr.startsWith('http://') || urlStr.startsWith('https://') || urlStr.startsWith('/')) {
      const {origin, protocol, hostname, port} = window.location;
      const url = new URL(urlStr, origin);
      url.protocol = protocol;
      url.hostname = hostname;
      url.port = port || (protocol === 'https:' ? '443' : '80');
      return url.toString();
    }
  } catch {}
  return urlStr;
}

window.customElements.define('gitea-origin-url', class extends HTMLElement {
  connectedCallback() {
    this.textContent = toOriginUrl(this.getAttribute('data-url'));
  }
});
