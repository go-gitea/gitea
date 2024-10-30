export function toAbsoluteLocaleDate(date: string, lang?: string, opts?: Intl.DateTimeFormatOptions) {
  // only use the date part, it is guaranteed to be in ISO format (YYYY-MM-DDTHH:mm:ss.sssZ) or (YYYY-MM-DD)
  // if there is an "Invalid Date" error, there must be something wrong in code and should be fixed.
  const dateSep = date.indexOf('T');
  date = dateSep === -1 ? date : date.substring(0, dateSep);
  return new Date(`${date}T00:00:00`).toLocaleString(lang || [], opts);
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

    if (!this.shadowRoot) this.attachShadow({mode: 'open'});
    this.shadowRoot.textContent = toAbsoluteLocaleDate(this.getAttribute('date'), lang, opt);
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
