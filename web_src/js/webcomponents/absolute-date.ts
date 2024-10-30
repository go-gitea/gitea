export function toAbsoluteLocaleDate(date: string, lang: string, opts: Intl.DateTimeFormatOptions) {
  return new Date(date).toLocaleString(lang || [], opts);
}

window.customElements.define('absolute-date', class extends HTMLElement {
  static observedAttributes = ['date', 'year', 'month', 'weekday', 'day'];

  initialized = false;

  update = () => {
    const opt: Intl.DateTimeFormatOptions = {};
    for (const attr of ['year', 'month', 'weekday', 'day']) {
      if (this.getAttribute(attr)) opt[attr] = this.getAttribute(attr);
    }
    const lang = this.closest('[lang]')?.getAttribute('lang') ||
      this.ownerDocument.documentElement.getAttribute('lang') || '';

    // only use the date part, it is guaranteed to be in ISO format (YYYY-MM-DDTHH:mm:ss.sssZ)
    let date = this.getAttribute('date');
    let dateSep = date.indexOf('T');
    dateSep = dateSep === -1 ? date.indexOf(' ') : dateSep;
    date = dateSep === -1 ? date : date.substring(0, dateSep);

    if (!this.shadowRoot) this.attachShadow({mode: 'open'});
    this.shadowRoot.textContent = toAbsoluteLocaleDate(date, lang, opt);
  };

  attributeChangedCallback(_name, oldValue, newValue) {
    if (!this.initialized || oldValue === newValue) return;
    this.update();
  }

  connectedCallback() {
    this.initialized = false;
    this.update();
    this.initialized = true;
  }
});
