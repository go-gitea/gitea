import {toOriginUrl} from '../utils/url.ts';

if (!window.customElements.get('origin-url')) window.customElements.define('origin-url', class extends HTMLElement {
  connectedCallback() {
    this.textContent = toOriginUrl(this.getAttribute('data-url')!);
  }
});
