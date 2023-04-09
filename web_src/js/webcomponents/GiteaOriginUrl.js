// Convert an absolute or relative URL to an absolute URL with the current origin
window.customElements.define('gitea-origin-url', class extends HTMLElement {
  connectedCallback() {
    const urlStr = this.getAttribute('data-url');
    try {
      // only process absolute HTTP/HTTPS URL or relative URLs ('/xxx' or '//host/xxx')
      if (urlStr.startsWith('http://') || urlStr.startsWith('https://') || urlStr.startsWith('/')) {
        const url = new URL(urlStr, window.origin);
        url.protocol = window.location.protocol;
        url.host = window.location.host;
        this.textContent = url.toString();
        return;
      }
    } catch {}
    this.textContent = urlStr;
  }
});
