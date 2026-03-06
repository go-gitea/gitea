// Vendored and simplified from @github/relative-time-element
// with hourCycle support from PR #329

import {Duration, elapsedTime, getRelativeTimeUnit, isDuration, unitNames} from './duration.ts';
import type {Unit} from './duration.ts';

type Format = 'auto' | 'datetime' | 'relative' | 'duration';
type ResolvedFormat = 'datetime' | 'relative' | 'duration';
type FormatStyle = 'long' | 'short' | 'narrow';
type Tense = 'auto' | 'past' | 'future';

const emptyDuration = new Duration();

let cachedBrowser12hCycle: boolean | undefined;
function isBrowser12hCycle(): boolean {
  return cachedBrowser12hCycle ??= new Intl.DateTimeFormat(undefined, {hour: 'numeric'})
    .resolvedOptions().hourCycle === 'h12';
}

function getUnitFactor(el: RelativeTime): number {
  if (!el.date) return Infinity;
  if (el.format === 'duration') {
    const precision = el.precision;
    if (precision === 'second') return 1000;
    if (precision === 'minute') return 60 * 1000;
  }
  const ms = Math.abs(Date.now() - el.date.getTime());
  if (ms < 60 * 1000) return 1000;
  if (ms < 60 * 60 * 1000) return 60 * 1000;
  return 60 * 60 * 1000;
}

const dateObserver = new (class {
  elements = new Set<RelativeTime>();
  time = Infinity;
  timer = -1;

  observe(element: RelativeTime): void {
    if (this.elements.has(element)) return;
    this.elements.add(element);
    const date = element.date;
    if (date?.getTime()) {
      const ms = getUnitFactor(element);
      const time = Date.now() + ms;
      if (time < this.time) {
        clearTimeout(this.timer);
        this.timer = window.setTimeout(() => this.update(), ms);
        this.time = time;
      }
    }
  }

  unobserve(element: RelativeTime): void {
    if (!this.elements.has(element)) return;
    this.elements.delete(element);
    if (!this.elements.size) {
      clearTimeout(this.timer);
      this.time = Infinity;
    }
  }

  update(): void {
    clearTimeout(this.timer);
    if (!this.elements.size) return;
    let nearestDistance = Infinity;
    for (const timeEl of this.elements) {
      nearestDistance = Math.min(nearestDistance, getUnitFactor(timeEl));
      timeEl.update();
    }
    this.time = Math.min(60 * 60 * 1000, nearestDistance);
    this.timer = window.setTimeout(() => this.update(), this.time);
    this.time += Date.now();
  }
})();

class RelativeTime extends HTMLElement {
  #customTitle = false;
  #updating = false;
  #renderRoot: ShadowRoot | HTMLElement;

  constructor() {
    super();
    this.#renderRoot = this.shadowRoot || this.attachShadow?.({mode: 'open'}) || this;
  }

  static get observedAttributes(): string[] {
    return [
      'second', 'minute', 'hour', 'weekday', 'day', 'month', 'year',
      'time-zone-name', 'prefix', 'threshold', 'tense', 'precision',
      'format', 'format-style', 'no-title', 'datetime', 'lang', 'title',
      'time-zone', 'hour-cycle',
    ];
  }

  get hourCycle(): string | undefined {
    const hc = this.closest('[hour-cycle]')?.getAttribute('hour-cycle') ||
      this.ownerDocument.documentElement.getAttribute('hour-cycle');
    if (hc === 'h11' || hc === 'h12' || hc === 'h23' || hc === 'h24') return hc;
    return isBrowser12hCycle() ? 'h12' : 'h23';
  }

  get #lang(): string {
    const lang = this.closest('[lang]')?.getAttribute('lang') ||
      this.ownerDocument.documentElement.getAttribute('lang');
    try {
      return new Intl.Locale(lang ?? '').toString();
    } catch {
      return 'default';
    }
  }

  get timeZone(): string | undefined {
    const tz = this.closest('[time-zone]')?.getAttribute('time-zone') ||
      this.ownerDocument.documentElement.getAttribute('time-zone');
    return tz || undefined;
  }

  get second(): 'numeric' | '2-digit' | undefined {
    const v = this.getAttribute('second');
    if (v === 'numeric' || v === '2-digit') return v;
    return undefined;
  }

  set second(value: string | undefined) {
    this.setAttribute('second', value || '');
  }

  get minute(): 'numeric' | '2-digit' | undefined {
    const v = this.getAttribute('minute');
    if (v === 'numeric' || v === '2-digit') return v;
    return undefined;
  }

  set minute(value: string | undefined) {
    this.setAttribute('minute', value || '');
  }

  get hour(): 'numeric' | '2-digit' | undefined {
    const v = this.getAttribute('hour');
    if (v === 'numeric' || v === '2-digit') return v;
    return undefined;
  }

  set hour(value: string | undefined) {
    this.setAttribute('hour', value || '');
  }

  get weekday(): 'long' | 'short' | 'narrow' | undefined {
    const weekday = this.getAttribute('weekday');
    if (weekday === 'long' || weekday === 'short' || weekday === 'narrow') return weekday;
    if (this.format === 'datetime' && weekday !== '') return this.formatStyle;
    return undefined;
  }

  set weekday(value: string | undefined) {
    this.setAttribute('weekday', value || '');
  }

  get day(): 'numeric' | '2-digit' | undefined {
    const day = this.getAttribute('day') ?? 'numeric';
    if (day === 'numeric' || day === '2-digit') return day;
    return undefined;
  }

  set day(value: string | undefined) {
    this.setAttribute('day', value || '');
  }

  get month(): 'numeric' | '2-digit' | 'short' | 'long' | 'narrow' | undefined {
    const format = this.format;
    let month = this.getAttribute('month');
    if (month === '') return undefined;
    month ??= format === 'datetime' ? this.formatStyle : 'short';
    if (month === 'numeric' || month === '2-digit' || month === 'short' || month === 'long' || month === 'narrow') {
      return month;
    }
    return undefined;
  }

  set month(value: string | undefined) {
    this.setAttribute('month', value || '');
  }

  get year(): 'numeric' | '2-digit' | undefined {
    const year = this.getAttribute('year');
    if (year === 'numeric' || year === '2-digit') return year;
    if (!this.hasAttribute('year') && new Date().getUTCFullYear() !== this.date?.getUTCFullYear()) {
      return 'numeric';
    }
    return undefined;
  }

  set year(value: string | undefined) {
    this.setAttribute('year', value || '');
  }

  get timeZoneName(): 'long' | 'short' | 'shortOffset' | 'longOffset' | 'shortGeneric' | 'longGeneric' | undefined {
    const name = this.getAttribute('time-zone-name');
    if (name === 'long' || name === 'short' || name === 'shortOffset' ||
        name === 'longOffset' || name === 'shortGeneric' || name === 'longGeneric') {
      return name;
    }
    return undefined;
  }

  set timeZoneName(value: string | undefined) {
    this.setAttribute('time-zone-name', value || '');
  }

  get prefix(): string {
    return this.getAttribute('prefix') ?? (this.format === 'datetime' ? '' : 'on');
  }

  set prefix(value: string) {
    this.setAttribute('prefix', value);
  }

  get threshold(): string {
    const threshold = this.getAttribute('threshold');
    return threshold && isDuration(threshold) ? threshold : 'P30D';
  }

  set threshold(value: string) {
    this.setAttribute('threshold', value);
  }

  get tense(): Tense {
    const tense = this.getAttribute('tense');
    if (tense === 'past') return 'past';
    if (tense === 'future') return 'future';
    return 'auto';
  }

  set tense(value: string) {
    this.setAttribute('tense', value);
  }

  get precision(): Unit {
    const precision = this.getAttribute('precision');
    if ((unitNames as readonly string[]).includes(precision!)) return precision as Unit;
    return 'second';
  }

  set precision(value: string) {
    this.setAttribute('precision', value);
  }

  get format(): Format {
    const format = this.getAttribute('format');
    if (format === 'datetime') return 'datetime';
    if (format === 'relative') return 'relative';
    if (format === 'duration') return 'duration';
    return 'auto';
  }

  set format(value: string) {
    this.setAttribute('format', value);
  }

  get formatStyle(): FormatStyle {
    const formatStyle = this.getAttribute('format-style');
    if (formatStyle === 'long') return 'long';
    if (formatStyle === 'short') return 'short';
    if (formatStyle === 'narrow') return 'narrow';
    if (this.format === 'datetime') return 'short';
    return 'long';
  }

  set formatStyle(value: string) {
    this.setAttribute('format-style', value);
  }

  get noTitle(): boolean {
    return this.hasAttribute('no-title');
  }

  set noTitle(value: boolean) {
    this.toggleAttribute('no-title', value);
  }

  get datetime(): string {
    return this.getAttribute('datetime') || '';
  }

  set datetime(value: string) {
    this.setAttribute('datetime', value);
  }

  get date(): Date | null {
    const parsed = Date.parse(this.datetime);
    return Number.isNaN(parsed) ? null : new Date(parsed);
  }

  set date(value: Date | null) {
    this.datetime = value?.toISOString() || '';
  }

  connectedCallback(): void {
    this.update();
  }

  disconnectedCallback(): void {
    dateObserver.unobserve(this);
  }

  attributeChangedCallback(attrName: string, oldValue: string | null, newValue: string | null): void {
    if (oldValue === newValue) return;
    if (attrName === 'title') {
      this.#customTitle = newValue !== null && (this.date && this.#getFormattedTitle(this.date)) !== newValue;
    }
    if (!this.#updating && !(attrName === 'title' && this.#customTitle)) {
      this.#updating = true;
      queueMicrotask(() => {
        this.update();
        this.#updating = false;
      });
    }
  }

  #getFormattedTitle(date: Date): string {
    return new Intl.DateTimeFormat(this.#lang, {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hourCycle: this.hourCycle as Intl.DateTimeFormatOptions['hourCycle'],
    }).format(date);
  }

  #resolveFormat(duration: Duration): ResolvedFormat {
    const format = this.format;
    if (format === 'datetime') return 'datetime';
    if (format === 'duration') return 'duration';
    if ((format === 'auto' || format === 'relative') && typeof Intl !== 'undefined' && Intl.RelativeTimeFormat) {
      const tense = this.tense;
      if (tense === 'past' || tense === 'future') return 'relative';
      if (Duration.compare(duration, this.threshold) === 1) return 'relative';
    }
    return 'datetime';
  }

  #getDurationFormat(duration: Duration): string {
    const locale = this.#lang;
    const style = this.formatStyle;
    const tense = this.tense;
    if ((tense === 'past' && duration.sign !== -1) || (tense === 'future' && duration.sign !== 1)) {
      duration = emptyDuration;
    }
    const display = `${this.precision}sDisplay`;
    if (duration.blank) {
      return emptyDuration.toLocaleString(locale, {style, [display]: 'always'});
    }
    return duration.abs().toLocaleString(locale, {style});
  }

  #getRelativeFormat(duration: Duration): string {
    const relativeFormat = new Intl.RelativeTimeFormat(this.#lang, {
      numeric: 'auto',
      style: this.formatStyle,
    });
    const tense = this.tense;
    if (tense === 'future' && duration.sign !== 1) duration = emptyDuration;
    if (tense === 'past' && duration.sign !== -1) duration = emptyDuration;
    const [int, unit] = getRelativeTimeUnit(duration);
    if (unit === 'second' && int < 10) {
      return relativeFormat.format(0, this.precision === 'millisecond' ? 'second' : this.precision);
    }
    return relativeFormat.format(int, unit);
  }

  #getDateTimeFormat(date: Date): string {
    const formatter = new Intl.DateTimeFormat(this.#lang, {
      second: this.second,
      minute: this.minute,
      hour: this.hour,
      weekday: this.weekday,
      day: this.day,
      month: this.month,
      year: this.year,
      timeZoneName: this.timeZoneName,
      timeZone: this.timeZone,
      hourCycle: this.hour ? this.hourCycle as Intl.DateTimeFormatOptions['hourCycle'] : undefined,
    });
    return `${this.prefix} ${formatter.format(date)}`.trim();
  }

  #updateRenderRootContent(content: string): void {
    const existing = this.#renderRoot.firstChild;
    if (existing instanceof HTMLSpanElement && existing.textContent === content) return;
    const span = document.createElement('span');
    span.setAttribute('part', 'root');
    span.textContent = content;
    (this.#renderRoot as Element).replaceChildren(span);
  }

  update(): void {
    const date = this.date;
    if (typeof Intl === 'undefined' || !Intl.DateTimeFormat || !date) {
      return;
    }
    const now = Date.now();
    if (!this.#customTitle) {
      const newTitle = this.#getFormattedTitle(date) || '';
      if (newTitle && !this.noTitle) this.setAttribute('title', newTitle);
    }
    const duration = elapsedTime(date, this.precision, now);
    const format = this.#resolveFormat(duration);
    let newText: string;
    if (format === 'duration') {
      newText = this.#getDurationFormat(duration);
    } else if (format === 'relative') {
      newText = this.#getRelativeFormat(duration);
    } else {
      newText = this.#getDateTimeFormat(date);
    }
    if (newText) {
      this.#updateRenderRootContent(newText);
    } else if (this.shadowRoot === this.#renderRoot && this.textContent) {
      this.#updateRenderRootContent(this.textContent);
    }
    if (format === 'relative' || format === 'duration') {
      dateObserver.observe(this);
    } else {
      dateObserver.unobserve(this);
    }
  }
}

// Register the custom element
window.customElements.define('relative-time', RelativeTime);
