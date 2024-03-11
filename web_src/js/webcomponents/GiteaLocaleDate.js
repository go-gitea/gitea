window.customElements.define('gitea-locale-date', class extends HTMLElement {
  static observedAttributes = ['date', 'year', 'month', 'weekday', 'day'];

  update = () => {
    const year = this.getAttribute('year') ?? '';
    const month = this.getAttribute('month') ?? '';
    const weekday = this.getAttribute('weekday') ?? '';
    const day = this.getAttribute('day') ?? '';
    const lang = this.closest('[lang]')?.getAttribute('lang') ||
      this.ownerDocument.documentElement.getAttribute('lang') ||
      '';
    const date = new Date(this.getAttribute('date'));

    // apply negative timezone offset because `new Date()` above assumes that `yyyy-mm-dd` is
    // a UTC date, so the local date will have a offset towards UTC which we reverse here.
    // Ref: https://stackoverflow.com/a/14569783/808699
    const correctedDate = new Date(date.getTime() - date.getTimezoneOffset() * -60000);

    if (!this.shadowRoot) this.attachShadow({mode: 'open'});
    this.shadowRoot.textContent = correctedDate.toLocaleString(lang ?? [], {
      ...(year && {year}),
      ...(month && {month}),
      ...(weekday && {weekday}),
      ...(day && {day}),
    });
  };

  attributeChangedCallback(_name, oldValue, newValue) {
    if (oldValue === newValue || !this.initialized) return;
    this.update();
  }

  connectedCallback() {
    this.initialized = false;
    this.update();
    this.initialized = true;
  }
});
