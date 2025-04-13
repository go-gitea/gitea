import {toOriginUrl} from '../utils/url.ts';

window.customElements.define('origin-url', class extends HTMLElement {
  connectedCallback() {
    this.textContent = toOriginUrl(this.getAttribute('data-url'));
  }
});
