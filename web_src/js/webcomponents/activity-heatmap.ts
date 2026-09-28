// ActivityHeatmap Web Component
// Converted from Vue component to reduce framework dependency

import tippy, {createSingleton} from 'tippy.js';
import type {CreateSingletonInstance, Instance} from 'tippy.js';

type HeatmapValue = {date: Date; count: number};

type HeatmapLocale = {
  textTotalContributions: string;
  heatMapLocale: {
    months: string[];
    days: string[];
    on: string;
    more: string;
    less: string;
  };
  noDataText: string;
  tooltipUnit: string;
};

const colorRange = [
  'var(--color-secondary-alpha-60)',
  'var(--color-primary-light-4)',
  'var(--color-primary-light-2)',
  'var(--color-primary)',
  'var(--color-primary-dark-2)',
  'var(--color-primary-dark-4)',
];

const squareSize = 10;
const squareBorder = 2;
const cellSize = squareSize + squareBorder;
const daysInWeek = 7;
const trailingDays = 365;
const gridLeft = Math.ceil(squareSize * 2.5);
const gridTop = squareSize + squareSize / 2;

type HeatmapCell = {date: Date; colorIndex: number; ariaLabel: string; tooltip: string};
type MonthLabel = {monthIdx: number; weekIdx: number};
type DayLabel = {dayIdx: number; rowIdx: number};

type ComputedGrid = {
  calendar: HeatmapCell[][];
  monthLabels: MonthLabel[];
  dayLabels: DayLabel[];
  width: number;
  height: number;
};

function dateKey(d: Date): string {
  return `${d.getFullYear()}${String(d.getMonth()).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`;
}

function shiftDate(d: Date, days: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + days);
  return out;
}

function getWeekFirstDay(): number {
  const userLocale = navigator.language || 'en-US';
  try {
    const localeInfo = new Intl.Locale(userLocale);
    const weekInfo = localeInfo.getWeekInfo();
    return weekInfo.firstDay;
  } catch {
    const region = userLocale.split('-')[1];
    const sundayRegions = ['US', 'CA', 'MX', 'JP', 'KR', 'IL', 'SA', 'IN', 'BR'];
    return !region || sundayRegions.includes(region) ? 7 : 1;
  }
}

function computeGrid(values: HeatmapValue[], locale: HeatmapLocale, now: Date): ComputedGrid {
  const start = shiftDate(now, -trailingDays);
  const firstDayIdx = getWeekFirstDay() % daysInWeek;
  const padStart = (start.getDay() - firstDayIdx + daysInWeek) % daysInWeek;
  const padEnd = (firstDayIdx - now.getDay() - 1 + daysInWeek) % daysInWeek;
  const weekCount = (trailingDays + 1 + padStart + padEnd) / daysInWeek;

  const maxCount = values.length ? Math.max(...values.map((v) => v.count)) : 0;
  const max = maxCount > 0 ? Math.ceil(maxCount / 5 * 4) : 1;

  const activities = new Map<string, {count: number; colorIndex: number}>();
  for (const {date, count} of values) {
    const colorIndex = count >= max ? 4 : Math.max(1, Math.ceil((count / max) * 3));
    activities.set(dateKey(date), {count, colorIndex});
  }

  const {on} = locale.heatMapLocale;
  const {noDataText, tooltipUnit} = locale;
  const currentLocale = navigator.language || 'en-US';

  const cursorStart = shiftDate(start, -padStart);
  const cursor = new Date(cursorStart.getFullYear(), cursorStart.getMonth(), cursorStart.getDate());
  const calendar: HeatmapCell[][] = [];
  for (let w = 0; w < weekCount; w++) {
    const week: HeatmapCell[] = [];
    for (let d = 0; d < daysInWeek; d++) {
      const hit = activities.get(dateKey(cursor));
      const dateStr = cursor.toLocaleDateString(currentLocale, {year: 'numeric', month: 'short', day: 'numeric'});
      const head = hit ? `${hit.count} ${tooltipUnit}` : noDataText;
      week.push({
        date: new Date(cursor),
        colorIndex: hit ? hit.colorIndex : 0,
        ariaLabel: `${head} ${on} ${dateStr}`,
        tooltip: `<b>${head}</b> ${on} ${dateStr}`,
      });
      cursor.setDate(cursor.getDate() + 1);
    }
    calendar.push(week);
  }

  const monthLabels: MonthLabel[] = [];
  for (let w = 1; w < calendar.length; w++) {
    const prev = calendar[w - 1][0].date;
    const curr = calendar[w][0].date;
    if (prev.getMonth() !== curr.getMonth()) {
      monthLabels.push({monthIdx: curr.getMonth(), weekIdx: w});
    }
  }

  const dayLabels: DayLabel[] = [];
  for (let i = 0; i < daysInWeek; i++) {
    const labelDay = firstDayIdx + i;
    if (labelDay % 2 === 0) continue;
    const dayIdx = labelDay % daysInWeek;
    dayLabels.push({dayIdx, rowIdx: i});
  }

  const width = gridLeft + (cellSize * weekCount) + squareBorder;
  const height = gridTop + (cellSize * daysInWeek);
  return {calendar, monthLabels, dayLabels, width, height};
}

const legendViewBox = `${cellSize} 0 ${squareSize * (colorRange.length + 2)} ${squareSize}`;

class ActivityHeatmap extends HTMLElement {
  #values: HeatmapValue[] = [];
  #locale: HeatmapLocale | null = null;
  #shadow!: ShadowRoot;
  #singleton: CreateSingletonInstance | null = null;
  #cellInstances = new Map<Element, Instance>();
  #boundLazyInitTooltip: (e: Event) => void;
  #boundHandleDayClick: (date: Date) => void;

  constructor() {
    super();
    this.#shadow = this.attachShadow({mode: 'open'});
    this.#boundLazyInitTooltip = this.#lazyInitTooltip.bind(this);
    this.#boundHandleDayClick = this.#handleDayClick.bind(this);
  }

  static get observedAttributes(): string[] {
    return ['data-values', 'data-locale'];
  }

  attributeChangedCallback(name: string, _old: string, newVal: string): void {
    if (name === 'data-values' && newVal) {
      try {
        this.#values = JSON.parse(newVal).map((v: {timestamp: number; count: number}) => ({
          date: new Date(v.timestamp * 1000),
          count: v.count,
        }));
      } catch {
        this.#values = [];
      }
    } else if (name === 'data-locale' && newVal) {
      try {
        this.#locale = JSON.parse(newVal);
      } catch {
        this.#locale = null;
      }
    }
    this.#render();
  }

  connectedCallback(): void {
    this.#singleton = createSingleton([], {
      overrides: [],
      moveTransition: 'transform 0.1s ease-out',
      allowHTML: true,
      theme: 'tooltip',
      role: 'tooltip',
      placement: 'top',
    });
    (this.#shadow as EventTarget).addEventListener('mouseover', this.#boundLazyInitTooltip);
    this.#render();
  }

  disconnectedCallback(): void {
    this.#singleton?.destroy();
    for (const instance of this.#cellInstances.values()) instance.destroy();
    this.#cellInstances.clear();
    (this.#shadow as EventTarget).removeEventListener('mouseover', this.#boundLazyInitTooltip);
  }

  #lazyInitTooltip(e: Event): void {
    const el = e.target as Element;
    if (!this.#singleton || this.#cellInstances.has(el) || !el.classList.contains('heatmap-day')) return;
    this.#cellInstances.set(el, tippy(el, {content: el.getAttribute('data-tooltip')!}));
    this.#singleton.setInstances([...this.#cellInstances.values()]);
  }

  #handleDayClick(date: Date): void {
    const params = new URLSearchParams(window.location.search);
    const queryDate = params.get('date');
    const clickedDate = new Date(date.getTime() - (date.getTimezoneOffset() * 60000)).toISOString().substring(0, 10);

    if (queryDate && queryDate === clickedDate) {
      params.delete('date');
    } else {
      params.set('date', clickedDate);
    }
    params.delete('page');

    const newSearch = params.toString();
    window.location.search = newSearch.length ? `?${newSearch}` : '';
  }

  #render(): void {
    if (!this.#locale) return;

    const now = new Date();
    const grid = computeGrid(this.#values, this.#locale, now);

    const styles = `
      <style>
        .heatmap-svg { display: block; }
        .heatmap-month-label { font-size: 10px; fill: var(--color-text-light-2); }
        .heatmap-day-label { font-size: 10px; fill: var(--color-text-light-2); }
        .heatmap-day { cursor: pointer; }
        .heatmap-footer { display: flex; align-items: center; gap: 8px; margin-top: 8px; font-size: 12px; color: var(--color-text-light-1); }
        .heatmap-legend { display: flex; align-items: center; gap: 4px; }
        .heatmap-legend-svg { display: block; }
      </style>
    `;

    const svg = `
      <svg class="heatmap-svg" viewBox="0 0 ${grid.width} ${grid.height}" xmlns="http://www.w3.org/2000/svg">
        <g class="heatmap-month-labels" transform="translate(${gridLeft}, 0)">
          ${grid.monthLabels.map((m) => `<text class="heatmap-month-label" x="${cellSize * m.weekIdx}" y="${cellSize - squareBorder}">${this.#locale!.heatMapLocale.months[m.monthIdx]}</text>`).join('')}
        </g>
        <g class="heatmap-day-labels" transform="translate(0, ${gridTop})">
          ${grid.dayLabels.map((day) => `<text class="heatmap-day-label" x="0" y="${day.rowIdx * cellSize + squareSize - squareBorder}">${this.#locale!.heatMapLocale.days[day.dayIdx]}</text>`).join('')}
        </g>
        <g class="heatmap-grid" transform="translate(${gridLeft}, ${gridTop})">
          ${grid.calendar.map((week, w) => `
            <g class="heatmap-week" transform="translate(${w * cellSize}, 0)">
              ${week.map((day, d) => day.date < now ? `
                <rect
                  class="heatmap-day"
                  transform="translate(0, ${d * cellSize})"
                  width="${squareSize}"
                  height="${squareSize}"
                  style="fill: ${colorRange[day.colorIndex]}"
                  aria-label="${day.ariaLabel}"
                  data-tooltip="${day.tooltip}"
                  data-date="${day.date.toISOString()}"
                />
              ` : '').join('')}
            </g>
          `).join('')}
        </g>
      </svg>
    `;

    const footer = `
      <div class="heatmap-footer">
        <div>${this.#locale.textTotalContributions}</div>
        <div class="heatmap-legend">
          <div>${this.#locale.heatMapLocale.less}</div>
          <svg class="heatmap-legend-svg" viewBox="${legendViewBox}" height="${squareSize}" xmlns="http://www.w3.org/2000/svg">
            ${colorRange.map((color, i) => `<rect width="${squareSize}" height="${squareSize}" x="${(i + 1) * cellSize}" style="fill: ${color}"/>`).join('')}
          </svg>
          <div>${this.#locale.heatMapLocale.more}</div>
        </div>
      </div>
    `;

    this.#shadow.innerHTML = styles + svg + footer;

    // Add click handlers to rect elements
    for (const rect of this.#shadow.querySelectorAll('.heatmap-day')) {
      rect.addEventListener('click', () => {
        const dateStr = rect.getAttribute('data-date');
        if (dateStr) this.#boundHandleDayClick(new Date(dateStr));
      });
    }
  }
}

if (!customElements.get('activity-heatmap')) {
  customElements.define('activity-heatmap', ActivityHeatmap);
}
