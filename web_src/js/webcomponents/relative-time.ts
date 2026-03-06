// Vendored and simplified from @github/relative-time-element
// with hourCycle support from PR #329 and part="root" from PR #341

// -- Duration format ponyfill and Duration class --

class ListFormatPonyFill {
  formatToParts(members: string[]): {type: string, value: string}[] {
    const parts: {type: string, value: string}[] = [];
    for (const value of members) {
      parts.push({type: 'element', value});
      parts.push({type: 'literal', value: ', '});
    }
    return parts.slice(0, -1);
  }
}

const ListFormat: any = (typeof Intl !== 'undefined' && Intl.ListFormat) || ListFormatPonyFill;

const partsTable: [string, string][] = [
  ['years', 'year'],
  ['months', 'month'],
  ['weeks', 'week'],
  ['days', 'day'],
  ['hours', 'hour'],
  ['minutes', 'minute'],
  ['seconds', 'second'],
  ['milliseconds', 'millisecond'],
];

const twoDigitFormatOptions = {minimumIntegerDigits: 2};

type DurationFormatStyle = 'long' | 'short' | 'narrow' | 'digital';
type DurationFormatOptions = Partial<{
  style: DurationFormatStyle;
  years: string;
  yearsDisplay: 'always' | 'auto';
  months: string;
  monthsDisplay: 'always' | 'auto';
  weeks: string;
  weeksDisplay: 'always' | 'auto';
  days: string;
  daysDisplay: 'always' | 'auto';
  hours: string;
  hoursDisplay: 'always' | 'auto';
  minutes: string;
  minutesDisplay: 'always' | 'auto';
  seconds: string;
  secondsDisplay: 'always' | 'auto';
  milliseconds: string;
  millisecondsDisplay: 'always' | 'auto';
  [key: string]: any;
}>;

class DurationFormat {
  #options: any;

  constructor(locale: string, options: DurationFormatOptions = {}) {
    let style = options.style || 'short';
    if (style !== 'long' && style !== 'short' && style !== 'narrow' && style !== 'digital') style = 'short';
    let prevStyle = style === 'digital' ? 'numeric' : style;
    const hours = options.hours || prevStyle;
    prevStyle = hours === '2-digit' ? 'numeric' : hours;
    const minutes = options.minutes || prevStyle;
    prevStyle = minutes === '2-digit' ? 'numeric' : minutes;
    const seconds = options.seconds || prevStyle;
    prevStyle = seconds === '2-digit' ? 'numeric' : seconds;
    const milliseconds = options.milliseconds || prevStyle;
    this.#options = {
      locale,
      style,
      years: options.years || (style === 'digital' ? 'short' : style),
      yearsDisplay: options.yearsDisplay === 'always' ? 'always' : 'auto',
      months: options.months || (style === 'digital' ? 'short' : style),
      monthsDisplay: options.monthsDisplay === 'always' ? 'always' : 'auto',
      weeks: options.weeks || (style === 'digital' ? 'short' : style),
      weeksDisplay: options.weeksDisplay === 'always' ? 'always' : 'auto',
      days: options.days || (style === 'digital' ? 'short' : style),
      daysDisplay: options.daysDisplay === 'always' ? 'always' : 'auto',
      hours,
      hoursDisplay: options.hoursDisplay === 'always' ? 'always' : style === 'digital' ? 'always' : 'auto',
      minutes,
      minutesDisplay: options.minutesDisplay === 'always' ? 'always' : style === 'digital' ? 'always' : 'auto',
      seconds,
      secondsDisplay: options.secondsDisplay === 'always' ? 'always' : style === 'digital' ? 'always' : 'auto',
      milliseconds,
      millisecondsDisplay: options.millisecondsDisplay === 'always' ? 'always' : 'auto',
    };
  }

  format(duration: Duration): string {
    const list: string[] = [];
    const options = this.#options;
    const style = options.style;
    const locale = options.locale;
    for (const [unit, nfUnit] of partsTable) {
      const value = (duration as any)[unit];
      if (options[`${unit}Display`] === 'auto' && !value) continue;
      const unitStyle = options[unit];
      const nfOpts: Intl.NumberFormatOptions = unitStyle === '2-digit' ?
        twoDigitFormatOptions :
        unitStyle === 'numeric' ?
          {} :
          {style: 'unit' as const, unit: nfUnit, unitDisplay: unitStyle};
      let formattedValue = new Intl.NumberFormat(locale, nfOpts).format(value);
      if (unit === 'months' && (unitStyle === 'narrow' || (style === 'narrow' && formattedValue.endsWith('m')))) {
        formattedValue = formattedValue.replace(/(\d+)m$/, '$1mo');
      }
      list.push(formattedValue);
    }
    return new ListFormat(locale, {
      type: 'unit',
      style: style === 'digital' ? 'short' : style,
    }).formatToParts(list).map((p: {value: string}) => p.value).join('');
  }
}

const durationRe = /^[-+]?P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$/;
const unitNames = ['year', 'month', 'week', 'day', 'hour', 'minute', 'second', 'millisecond'] as const;
type Unit = typeof unitNames[number];

function isDuration(str: string): boolean {
  return durationRe.test(str);
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
  readonly milliseconds: number;
  readonly sign: Sign;
  readonly blank: boolean;

  constructor(
    years = 0, months = 0, weeks = 0, days = 0,
    hours = 0, minutes = 0, seconds = 0, milliseconds = 0,
  ) {
    this.years = years || 0;
    this.months = months || 0;
    this.weeks = weeks || 0;
    this.days = days || 0;
    this.hours = hours || 0;
    this.minutes = minutes || 0;
    this.seconds = seconds || 0;
    this.milliseconds = milliseconds || 0;
    this.sign = (Math.sign(this.years) || Math.sign(this.months) || Math.sign(this.weeks) ||
      Math.sign(this.days) || Math.sign(this.hours) || Math.sign(this.minutes) ||
      Math.sign(this.seconds) || Math.sign(this.milliseconds)) as Sign;
    this.blank = this.sign === 0;
  }

  abs(): Duration {
    return new Duration(
      Math.abs(this.years), Math.abs(this.months), Math.abs(this.weeks), Math.abs(this.days),
      Math.abs(this.hours), Math.abs(this.minutes), Math.abs(this.seconds), Math.abs(this.milliseconds),
    );
  }

  static from(durationLike: unknown): Duration {
    if (typeof durationLike === 'string') {
      const str = durationLike.trim();
      const factor = str.startsWith('-') ? -1 : 1;
      const parsed = durationRe.exec(str)?.slice(1).map((x) => (Number(x) || 0) * factor);
      if (!parsed) return new Duration();
      return new Duration(...parsed);
    } else if (typeof durationLike === 'object') {
      const {years, months, weeks, days, hours, minutes, seconds, milliseconds} = durationLike as any;
      return new Duration(years, months, weeks, days, hours, minutes, seconds, milliseconds);
    }
    throw new RangeError('invalid duration');
  }

  static compare(one: unknown, two: unknown): -1 | 0 | 1 {
    const now = Date.now();
    const oneApplied = Math.abs(applyDuration(now, Duration.from(one)).getTime() - now);
    const twoApplied = Math.abs(applyDuration(now, Duration.from(two)).getTime() - now);
    return oneApplied > twoApplied ? -1 : oneApplied < twoApplied ? 1 : 0;
  }

  toLocaleString(locale: string, opts: DurationFormatOptions): string {
    return new DurationFormat(locale, opts).format(this);
  }
}

function applyDuration(date: Date | number, duration: Duration): Date {
  const r = new Date(date);
  if (duration.sign < 0) {
    r.setUTCSeconds(r.getUTCSeconds() + duration.seconds);
    r.setUTCMinutes(r.getUTCMinutes() + duration.minutes);
    r.setUTCHours(r.getUTCHours() + duration.hours);
    r.setUTCDate(r.getUTCDate() + duration.weeks * 7 + duration.days);
    r.setUTCMonth(r.getUTCMonth() + duration.months);
    r.setUTCFullYear(r.getUTCFullYear() + duration.years);
  } else {
    r.setUTCFullYear(r.getUTCFullYear() + duration.years);
    r.setUTCMonth(r.getUTCMonth() + duration.months);
    r.setUTCDate(r.getUTCDate() + duration.weeks * 7 + duration.days);
    r.setUTCHours(r.getUTCHours() + duration.hours);
    r.setUTCMinutes(r.getUTCMinutes() + duration.minutes);
    r.setUTCSeconds(r.getUTCSeconds() + duration.seconds);
  }
  return r;
}

function elapsedTime(date: Date, precision: Unit = 'second', now = Date.now()): Duration {
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
  const i = unitNames.indexOf(precision) || unitNames.length;
  return new Duration(
    i >= 0 ? year * sign : 0,
    i >= 1 ? (month - year * 12) * sign : 0,
    0,
    i >= 3 ? (day - month * 30) * sign : 0,
    i >= 4 ? (hr - day * 24) * sign : 0,
    i >= 5 ? (min - hr * 60) * sign : 0,
    i >= 6 ? (sec - min * 60) * sign : 0,
    i >= 7 ? (ms - sec * 1000) * sign : 0,
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
  let milliseconds = Math.abs(duration.milliseconds);
  if (milliseconds >= 900) seconds += Math.round(milliseconds / 1000);
  if (seconds || minutes || hours || days || weeks || months || years) milliseconds = 0;
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
  return new Duration(years * sign, months * sign, weeks * sign, days * sign, hours * sign, minutes * sign, seconds * sign, milliseconds * sign);
}

function getRelativeTimeUnit(duration: Duration, opts?: {relativeTo?: Date | number}): [number, Intl.RelativeTimeFormatUnit] {
  const rounded = roundToSingleUnit(duration, opts);
  if (rounded.blank) return [0, 'second'];
  for (const unit of unitNames) {
    if (unit === 'millisecond') continue;
    const val = (rounded as any)[`${unit}s`];
    if (val) return [val, unit];
  }
  return [0, 'second'];
}

// -- RelativeTime custom element --

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

window.customElements.define('relative-time', RelativeTime);
