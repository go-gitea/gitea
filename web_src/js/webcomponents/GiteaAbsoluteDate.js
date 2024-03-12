window.customElements.define('gitea-absolute-date', class extends HTMLElement {
  static observedAttributes = ['date', 'year', 'month', 'weekday', 'day'];

  update = () => {
    const year = this.getAttribute('year') ?? '';
    const month = this.getAttribute('month') ?? '';
    const weekday = this.getAttribute('weekday') ?? '';
    const day = this.getAttribute('day') ?? '';
    const lang = this.closest('[lang]')?.getAttribute('lang') ||
      this.ownerDocument.documentElement.getAttribute('lang') ||
      '';

    // only extract the `yyyy-mm-dd` part. When converting to Date, it will become midnight UTC and when rendered
    // as localized date, will have a offset towards UTC, which we remove to shift the timestamp to midnight in the
    // localized date. We should eventually use `Temporal.PlainDate` which will make the correction unnecessary.
    // - https://stackoverflow.com/a/14569783/808699
    // - https://tc39.es/proposal-temporal/docs/plaindate.html
    const date = new Date(this.getAttribute('date').substring(0, 10));
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
    if (!this.initialized || oldValue === newValue) return;
    this.update();
  }

  connectedCallback() {
    this.initialized = false;
    this.update();
    this.initialized = true;
  }
});
