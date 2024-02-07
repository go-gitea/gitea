// Convert an absolute or relative URL to an absolute URL with the current origin
export function toOriginUrl(urlStr) {
  try {
    // only process absolute HTTP/HTTPS URL or relative URLs ('/xxx' or '//host/xxx')
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
