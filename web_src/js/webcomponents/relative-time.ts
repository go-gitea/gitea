// Vendored and simplified from @github/relative-time-element@4.4.6
// https://github.com/github/relative-time-element
//
// MIT License
//
// Copyright (c) 2014-2018 GitHub, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

type FormatStyle = 'long' | 'short' | 'narrow';

const unitNames = ['year', 'month', 'week', 'day', 'hour', 'minute', 'second'] as const;

const durationRe = /^[-+]?P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$/;
const unixSecondsRe = /^\d+$/;

function parseDurationMs(str: string): number {
  const m = durationRe.exec(str);
  if (!m) return -1;
  const [, y, mo, w, d, h, min, s] = m.map(Number);
  return ((y || 0) * 365.25 + (mo || 0) * 30.44 + (w || 0) * 7 + (d || 0)) * 86400000 +
    ((h || 0) * 3600 + (min || 0) * 60 + (s || 0)) * 1000;
}

type Sign = -1 | 0 | 1;

class Duration {
  readonly years: number;
  readonly months: number;
  readonly weeks: number;
  readonly days: number;
  readonly hours: number;
  readonly minutes: number;
  readonly seconds: number;
  readonly sign: Sign;
  readonly blank: boolean;

  constructor(
    years = 0, months = 0, weeks = 0, days = 0,
    hours = 0, minutes = 0, seconds = 0,
  ) {
    this.years = years || 0;
    this.months = months || 0;
    this.weeks = weeks || 0;
    this.days = days || 0;
    this.hours = hours || 0;
    this.minutes = minutes || 0;
    this.seconds = seconds || 0;
    this.sign = (Math.sign(this.years) || Math.sign(this.months) || Math.sign(this.weeks) ||
      Math.sign(this.days) || Math.sign(this.hours) || Math.sign(this.minutes) ||
      Math.sign(this.seconds)) as Sign;
    this.blank = this.sign === 0;
  }

  abs(): Duration {
    return new Duration(
      Math.abs(this.years), Math.abs(this.months), Math.abs(this.weeks), Math.abs(this.days),
      Math.abs(this.hours), Math.abs(this.minutes), Math.abs(this.seconds),
    );
  }
}

function elapsedTime(date: Date, now = Date.now()): Duration {
  const delta = date.getTime() - now;
  if (delta === 0) return new Duration();
  const sign = Math.sign(delta);
  const ms = Math.abs(delta);
  const sec = Math.floor(ms / 1000);
  const min = Math.floor(sec / 60);
  const hr = Math.floor(min / 60);
  const day = Math.floor(hr / 24);
  const month = Math.floor(day / 30);
  const year = Math.floor(month / 12);
  return new Duration(
    year * sign,
    (month - year * 12) * sign,
    0,
    (day - month * 30) * sign,
    (hr - day * 24) * sign,
    (min - hr * 60) * sign,
    (sec - min * 60) * sign,
  );
}

function roundToSingleUnit(duration: Duration, {relativeTo = Date.now()}: {relativeTo?: Date | number} = {}): Duration {
  relativeTo = new Date(relativeTo);
  if (duration.blank) return duration;
  const sign = duration.sign;
  let years = Math.abs(duration.years);
  let months = Math.abs(duration.months);
  let weeks = Math.abs(duration.weeks);
  let days = Math.abs(duration.days);
  let hours = Math.abs(duration.hours);
  let minutes = Math.abs(duration.minutes);
  let seconds = Math.abs(duration.seconds);
  if (seconds >= 55) minutes += Math.round(seconds / 60);
  if (minutes || hours || days || weeks || months || years) seconds = 0;
  if (minutes >= 55) hours += Math.round(minutes / 60);
  if (hours || days || weeks || months || years) minutes = 0;
  if (days && hours >= 12) days += Math.round(hours / 24);
  if (!days && hours >= 21) days += Math.round(hours / 24);
  if (days || weeks || months || years) hours = 0;
  const currentYear = relativeTo.getFullYear();
  const currentMonth = relativeTo.getMonth();
  const currentDate = relativeTo.getDate();
  if (days >= 27 || years + months + days) {
    const newMonthDate = new Date(relativeTo);
    newMonthDate.setDate(1);
    newMonthDate.setMonth(currentMonth + months * sign + 1);
    newMonthDate.setDate(0);
    const monthDateCorrection = Math.max(0, currentDate - newMonthDate.getDate());
    const newDate = new Date(relativeTo);
    newDate.setFullYear(currentYear + years * sign);
    newDate.setDate(currentDate - monthDateCorrection);
    newDate.setMonth(currentMonth + months * sign);
    newDate.setDate(currentDate - monthDateCorrection + days * sign);
    const yearDiff = newDate.getFullYear() - relativeTo.getFullYear();
    const monthDiff = newDate.getMonth() - relativeTo.getMonth();
    const daysDiff = Math.abs(Math.round((Number(newDate) - Number(relativeTo)) / 86400000)) + monthDateCorrection;
    const monthsDiff = Math.abs(yearDiff * 12 + monthDiff);
    if (daysDiff < 27) {
      if (days >= 6) {
        weeks += Math.round(days / 7);
        days = 0;
      } else {
        days = daysDiff;
      }
      months = years = 0;
    } else if (monthsDiff <= 11) {
      months = monthsDiff;
      years = 0;
    } else {
      months = 0;
      years = yearDiff * sign;
    }
    if (months || years) days = 0;
  }
  if (years) months = 0;
  if (weeks >= 4) months += Math.round(weeks / 4);
  if (months || years) weeks = 0;
  if (days && weeks && !months && !years) {
    weeks += Math.round(days / 7);
    days = 0;
  }
  return new Duration(years * sign, months * sign, weeks * sign, days * sign, hours * sign, minutes * sign, seconds * sign);
}

function getRelativeTimeUnit(duration: Duration, opts?: {relativeTo?: Date | number}): [number, Intl.RelativeTimeFormatUnit] {
  const rounded = roundToSingleUnit(duration, opts);
  if (rounded.blank) return [0, 'second'];
  for (const unit of unitNames) {
    const val = (rounded as any)[`${unit}s`];
    if (val) return [val, unit];
  }
  return [0, 'second'];
}

type Format = 'auto' | 'datetime' | 'relative' | 'duration';
type ResolvedFormat = 'datetime' | 'relative' | 'duration';
type Tense = 'auto' | 'past' | 'future';

const emptyDuration = new Duration();

let cachedBrowser12hCycle: boolean | undefined;
function isBrowser12hCycle(): boolean {
  return cachedBrowser12hCycle ??= new Intl.DateTimeFormat(undefined, {hour: 'numeric'})
    .resolvedOptions().hourCycle === 'h12';
}

function getUnitFactor(el: RelativeTime): number {
  if (!el.date) return Infinity;
  if (el.format === 'duration') return 1000;
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
    this.elements.add(element);
    const date = element.date;
    if (date && !Number.isNaN(date.getTime())) {
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
  static observedAttributes = [
    'second', 'minute', 'hour', 'weekday', 'day', 'month', 'year',
    'prefix', 'threshold', 'tense', 'format', 'format-style',
    'datetime', 'lang', 'hour-cycle',
  ];

  #updating = false;
  #renderRoot: ShadowRoot | HTMLElement;
  #span = document.createElement('span');

  constructor() { // eslint-disable-line wc/no-constructor -- shadow DOM setup requires constructor
    super();
    this.#renderRoot = this.shadowRoot || this.attachShadow?.({mode: 'open'}) || this;
    this.#span.setAttribute('part', 'root');
    this.#renderRoot.replaceChildren(this.#span);
  }

  get hourCycle(): string | undefined {
    const hc = this.closest('[hour-cycle]')?.getAttribute('hour-cycle');
    if (hc === 'h11' || hc === 'h12' || hc === 'h23' || hc === 'h24') return hc;
    return isBrowser12hCycle() ? 'h12' : 'h23';
  }

  get #lang(): string {
    for (const candidate of [this.closest('[lang]')?.getAttribute('lang'), navigator.language]) {
      if (!candidate) continue;
      try {
        return String(new Intl.Locale(candidate));
      } catch {}
    }
    return 'en';
  }

  get second(): 'numeric' | '2-digit' | undefined {
    const v = this.getAttribute('second');
    if (v === 'numeric' || v === '2-digit') return v;
    return undefined;
  }

  get minute(): 'numeric' | '2-digit' | undefined {
    const v = this.getAttribute('minute');
    if (v === 'numeric' || v === '2-digit') return v;
    return undefined;
  }

  get hour(): 'numeric' | '2-digit' | undefined {
    const v = this.getAttribute('hour');
    if (v === 'numeric' || v === '2-digit') return v;
    return undefined;
  }

  get weekday(): 'long' | 'short' | 'narrow' | undefined {
    const weekday = this.getAttribute('weekday');
    if (weekday === 'long' || weekday === 'short' || weekday === 'narrow') return weekday;
    if (this.format === 'datetime' && weekday !== '') return this.formatStyle;
    return undefined;
  }

  get day(): 'numeric' | '2-digit' | undefined {
    const day = this.getAttribute('day') ?? 'numeric';
    if (day === 'numeric' || day === '2-digit') return day;
    return undefined;
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

  get year(): 'numeric' | '2-digit' | undefined {
    const year = this.getAttribute('year');
    if (year === 'numeric' || year === '2-digit') return year;
    if (!this.hasAttribute('year') && new Date().getFullYear() !== this.date?.getFullYear()) {
      return 'numeric';
    }
    return undefined;
  }

  get prefix(): string {
    return this.getAttribute('prefix') ?? (this.format === 'datetime' ? '' : 'on');
  }

  set prefix(v: string) {
    this.setAttribute('prefix', v);
  }

  get #thresholdMs(): number {
    const ms = parseDurationMs(this.getAttribute('threshold') ?? '');
    return ms >= 0 ? ms : 30 * 86400000;
  }

  get tense(): Tense {
    const tense = this.getAttribute('tense');
    if (tense === 'past') return 'past';
    if (tense === 'future') return 'future';
    return 'auto';
  }

  get format(): Format {
    const format = this.getAttribute('format');
    if (format === 'datetime') return 'datetime';
    if (format === 'relative') return 'relative';
    if (format === 'duration') return 'duration';
    return 'auto';
  }

  get formatStyle(): FormatStyle {
    const formatStyle = this.getAttribute('format-style');
    if (formatStyle === 'long') return 'long';
    if (formatStyle === 'short') return 'short';
    if (formatStyle === 'narrow') return 'narrow';
    if (this.format === 'datetime') return 'short';
    return 'long';
  }

  get datetime(): string {
    return this.getAttribute('datetime') || '';
  }

  set datetime(v: string) {
    this.setAttribute('datetime', v);
  }

  get date(): Date | null {
    const dt = this.datetime;
    const parsed = unixSecondsRe.test(dt) ? Number(dt) * 1000 : Date.parse(dt);
    return Number.isNaN(parsed) ? null : new Date(parsed);
  }

  connectedCallback(): void {
    this.update();
  }

  disconnectedCallback(): void {
    dateObserver.unobserve(this);
  }

  attributeChangedCallback(_attrName: string, oldValue: string | null, newValue: string | null): void {
    if (oldValue === newValue) return;
    if (!this.#updating) {
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

  #resolveFormat(elapsedMs: number): ResolvedFormat {
    const format = this.format;
    if (format === 'datetime') return 'datetime';
    if (format === 'duration') return 'duration';
    if ((format === 'auto' || format === 'relative') && typeof Intl !== 'undefined' && Intl.RelativeTimeFormat) {
      const tense = this.tense;
      if (tense === 'past' || tense === 'future') return 'relative';
      if (elapsedMs < this.#thresholdMs) return 'relative';
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
    const d = duration.blank ? emptyDuration : duration.abs();
    if (typeof Intl !== 'undefined' && (Intl as any).DurationFormat) {
      const opts: Record<string, string> = {style};
      if (duration.blank) opts.secondsDisplay = 'always';
      return new (Intl as any).DurationFormat(locale, opts).format({
        years: d.years, months: d.months, weeks: d.weeks, days: d.days,
        hours: d.hours, minutes: d.minutes, seconds: d.seconds,
      });
    }
    // Fallback for browsers without Intl.DurationFormat
    const parts: string[] = [];
    for (const unit of unitNames) {
      const value = d[`${unit}s` as keyof Duration] as number;
      if (value || (duration.blank && unit === 'second')) {
        try {
          parts.push(new Intl.NumberFormat(locale, {style: 'unit', unit, unitDisplay: style}).format(value));
        } catch { // PaleMoon lacks Intl.NumberFormat unit style support
          parts.push(`${value} ${value === 1 ? unit : `${unit}s`}`);
        }
      }
    }
    return parts.join(style === 'narrow' ? ' ' : ', ');
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
      return relativeFormat.format(0, 'second');
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
      hourCycle: this.hour ? this.hourCycle as Intl.DateTimeFormatOptions['hourCycle'] : undefined,
    });
    return `${this.prefix} ${formatter.format(date)}`.trim();
  }

  update(): void {
    const date = this.date;
    if (typeof Intl === 'undefined' || !Intl.DateTimeFormat || !date) {
      dateObserver.unobserve(this);
      return;
    }
    const now = Date.now();
    const tooltip = this.#getFormattedTitle(date);
    if (tooltip && this.getAttribute('data-tooltip-content') !== tooltip) {
      this.setAttribute('data-tooltip-content', tooltip);
      this.setAttribute('aria-label', tooltip);
    }
    const elapsedMs = Math.abs(date.getTime() - now);
    const duration = elapsedTime(date, now);
    const format = this.#resolveFormat(elapsedMs);
    let newText: string;
    if (format === 'duration') {
      newText = this.#getDurationFormat(duration);
    } else if (format === 'relative') {
      newText = this.#getRelativeFormat(duration);
    } else {
      newText = this.#getDateTimeFormat(date);
    }
    newText ||= (this.shadowRoot === this.#renderRoot && this.textContent) || '';
    if (this.#span.textContent !== newText) this.#span.textContent = newText;
    if (format === 'relative' || format === 'duration') {
      dateObserver.observe(this);
    } else {
      dateObserver.unobserve(this);
    }
  }
}

window.customElements.define('relative-time', RelativeTime);
