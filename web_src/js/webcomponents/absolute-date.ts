export function toAbsoluteLocaleDate(date: string, lang?: string, opts?: Intl.DateTimeFormatOptions) {
  // only use the date part, it is guaranteed to be in ISO format (YYYY-MM-DDTHH:mm:ss.sssZ) or (YYYY-MM-DD)
  // if there is an "Invalid Date" error, there must be something wrong in code and should be fixed.
  // TODO: there is a root problem in backend code: the date "YYYY-MM-DD" is passed to backend without timezone (eg: deadline),
  // then backend parses it in server's timezone and stores the parsed timestamp into database.
  // If the user's timezone is different from the server's, the date might be displayed in the wrong day.
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

  attributeChangedCallback(_name: string, oldValue: string | null, newValue: string | null) {
    if (!this.initialized || oldValue === newValue) return;
    this.update();
  }

  connectedCallback() {
    this.initialized = false;
    this.update();
    this.initialized = true;
  }
});
