import {Temporal} from 'temporal-polyfill';

export function toAbsoluteLocaleDate(dateStr: string, lang: string, opts: Intl.DateTimeFormatOptions) {
  return Temporal.PlainDate.from(dateStr).toLocaleString(lang ?? [], opts);
}

window.customElements.define('absolute-date', class extends HTMLElement {
  static observedAttributes = ['date', 'year', 'month', 'weekday', 'day'];
  initialized: boolean;

  update = () => {
    const year = (this.getAttribute('year') ?? '') as Intl.DateTimeFormatOptions['year'];
    const month = (this.getAttribute('month') ?? '') as Intl.DateTimeFormatOptions['month'];
    const weekday = (this.getAttribute('weekday') ?? '') as Intl.DateTimeFormatOptions['weekday'];
    const day = (this.getAttribute('day') ?? '') as Intl.DateTimeFormatOptions['day'];
    const lang = this.closest('[lang]')?.getAttribute('lang') ||
      this.ownerDocument.documentElement.getAttribute('lang') || '';

    // only use the first 10 characters, e.g. the `yyyy-mm-dd` part
    const dateStr = this.getAttribute('date').substring(0, 10);

    if (!this.shadowRoot) this.attachShadow({mode: 'open'});
    this.shadowRoot.textContent = toAbsoluteLocaleDate(dateStr, lang, {
      ...(year && {year}),
      ...(month && {month}),
      ...(weekday && {weekday}),
      ...(day && {day}),
    });
  };

  attributeChangedCallback(_name: string, oldValue: any, newValue: any) {
    if (!this.initialized || oldValue === newValue) return;
    this.update();
  }

  connectedCallback() {
    this.initialized = false;
    this.update();
    this.initialized = true;
  }
});
