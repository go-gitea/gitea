<script lang="ts" setup>
import {computed, onBeforeUnmount, onMounted, ref, shallowRef, useTemplateRef, watch} from 'vue';
import tippy, {createSingleton} from 'tippy.js';
import type {CreateSingletonInstance, Instance} from 'tippy.js';
import {getCurrentLocale} from '../utils.ts';
import {html} from '../utils/html.ts';
import {GET} from '../modules/fetch.ts';
import {trString} from '../modules/i18n.ts';
import {showErrorToast} from '../modules/toast.ts';
import SvgIcon from './SvgIcon.vue';

type HeatmapData = {values: Array<{date: Date; count: number}>; total: number};
type HeatmapCell = {date: Date; visible: boolean; colorIndex: number; ariaLabel: string; tooltip: string};
type MonthLabel = {monthIdx: number; weekIdx: number};
type DayLabel = {dayIdx: number; rowIdx: number};

const props = defineProps<{
  initialYear: number; // 0 means the last 12 months
  initialData: HeatmapData;
  loadData: (year: number) => Promise<HeatmapData>;
  firstYear: number;
  locale: {
    textTotalContributions: string;
    heatMapLocale: {months: string[]; days: string[]; on: string; more: string; less: string};
    noDataText: string;
    tooltipUnit: string;
    last12Months: string;
    timeRange: string;
    previousMonths: string;
    nextMonths: string;
  };
}>();

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

const now = new Date();
const currentYear = now.getFullYear();
const lastYear = Math.max(currentYear, props.initialYear);
const firstYear = Math.min(props.firstYear || currentYear, props.initialYear || currentYear);
const years = [0];
for (let optionYear = lastYear; optionYear >= firstYear; optionYear--) {
  years.push(optionYear);
}

const year = ref(props.initialYear);
const data = shallowRef(props.initialData);
const isLoading = ref(false);

const elRoot = useTemplateRef('elRoot');
const containerWidth = ref(0);
let resizeObserver: ResizeObserver | null = null;

let latestFilterId = 0;

async function loadFeedHtml(params: URLSearchParams) {
  const feedParams = new URLSearchParams(params);
  feedParams.set('div-only', 'true');
  const resp = await GET(`${window.location.pathname}?${feedParams}`);
  if (!resp.ok) throw new Error(`Failed to load activity feed: ${resp.status} ${resp.statusText}`);
  return resp.text();
}

async function applyFilter(params: URLSearchParams, newYear: number) {
  const filterId = ++latestFilterId;
  params.delete('page');
  const elFeed = document.querySelector('#activity-feed')!;
  elFeed.classList.add('is-loading');
  isLoading.value = true;
  try {
    const [feedHtml, newData] = await Promise.all([
      loadFeedHtml(params),
      newYear === year.value ? data.value : props.loadData(newYear),
    ]);
    if (filterId !== latestFilterId) return;
    elFeed.outerHTML = feedHtml;
    data.value = newData;
    year.value = newYear;
    const newSearch = params.toString();
    window.history.replaceState(null, '', newSearch ? `?${newSearch}` : window.location.pathname);
  } catch (err) {
    if (filterId === latestFilterId) showErrorToast(String(err));
  } finally {
    if (filterId === latestFilterId) {
      elFeed.classList.remove('is-loading');
      isLoading.value = false;
    }
  }
}

async function onYearChange(event: Event) {
  const elSelect = event.target as HTMLSelectElement;
  const newYear = Number(elSelect.value);
  const params = new URLSearchParams(window.location.search);
  if (newYear) {
    params.set('year', String(newYear));
  } else {
    params.delete('year');
  }
  params.delete('date');
  await applyFilter(params, newYear);
  elSelect.value = String(year.value); // reverts a failed or superseded change
}

function dateKey(d: Date): string {
  return `${d.getFullYear()}${String(d.getMonth()).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`;
}

function shiftDate(d: Date, days: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + days);
  return out;
}

function getWeekFirstDay() {
  const userLocale = navigator.language || 'en-US';
  try {
    // Intl.Locale.prototype.getWeekInfo() is still "Limited availability"
    const localeInfo = new Intl.Locale(userLocale);
    const weekInfo = localeInfo.getWeekInfo();
    return weekInfo.firstDay;
  } catch {
    // Fallback: countries that widely use Sunday as the first day of the week
    const region = userLocale.split('-')[1];
    const sundayRegions = ['US', 'CA', 'MX', 'JP', 'KR', 'IL', 'SA', 'IN', 'BR'];
    return !region || sundayRegions.includes(region) ? 7 : 1;
  }
}

const firstDayIdx = getWeekFirstDay() % daysInWeek; // 0 to 6

const dayLabels: DayLabel[] = [];
for (let rowIdx = 0; rowIdx < daysInWeek; rowIdx++) {
  const labelDay = firstDayIdx + rowIdx;
  if (labelDay % 2 === 0) continue; // only show "Mon/Wed/Fri/Sun" because of limited vertical space
  dayLabels.push({dayIdx: labelDay % daysInWeek, rowIdx});
}

const calendar = computed(() => {
  const start = year.value ? new Date(year.value, 0, 1) : shiftDate(now, -trailingDays);
  const end = year.value ? new Date(year.value, 11, 31) : now;
  const padStart = (start.getDay() - firstDayIdx + daysInWeek) % daysInWeek;

  const {values} = data.value;
  const maxCount = values.length ? Math.max(...values.map((value) => value.count)) : 0;
  const max = maxCount > 0 ? Math.ceil(maxCount / 5 * 4) : 1;

  const activities = new Map<string, {count: number; colorIndex: number}>();
  for (const {date, count} of values) {
    const colorIndex = count >= max ? 4 : Math.max(1, Math.ceil((count / max) * 3));
    activities.set(dateKey(date), {count, colorIndex});
  }

  const {on} = props.locale.heatMapLocale;
  const {noDataText, tooltipUnit} = props.locale;
  const dateFormat = new Intl.DateTimeFormat(getCurrentLocale(), {year: 'numeric', month: 'short', day: 'numeric'});

  let cursor = new Date(start.getFullYear(), start.getMonth(), start.getDate() - padStart);
  const weeks: HeatmapCell[][] = [];
  while (cursor.getTime() <= end.getTime()) {
    const week: HeatmapCell[] = [];
    for (let d = 0; d < daysInWeek; d++) {
      const hit = activities.get(dateKey(cursor));
      const dateStr = dateFormat.format(cursor);
      const head = hit ? `${hit.count} ${tooltipUnit}` : noDataText;
      week.push({
        date: cursor,
        visible: year.value ? cursor.getFullYear() === year.value : cursor < now,
        colorIndex: hit ? hit.colorIndex : 0,
        ariaLabel: `${head} ${on} ${dateStr}`,
        tooltip: html`<b>${head}</b> ${on} ${dateStr}`,
      });
      cursor = new Date(cursor.getFullYear(), cursor.getMonth(), cursor.getDate() + 1); // recreated to stay at midnight across DST
    }
    weeks.push(week);
  }
  return weeks;
});

const weeksPerPage = computed(() => {
  if (!containerWidth.value) return calendar.value.length;
  return Math.min(calendar.value.length, Math.max(1, Math.floor((containerWidth.value - gridLeft - squareBorder) / cellSize)));
});
const weeks = computed(() => {
  const pastWeekCount = calendar.value.filter((week) => week[0].date <= now).length;
  return calendar.value.slice(0, Math.max(pastWeekCount, weeksPerPage.value)); // a range wider than the container drops its future weeks
});
const maxWeeksBack = computed(() => weeks.value.length - weeksPerPage.value);
const weeksBack = ref(0);
watch(weeks, () => {
  weeksBack.value = 0;
});
const pageStart = computed(() => maxWeeksBack.value - weeksBack.value);

function changePage(step: number) {
  weeksBack.value = Math.min(maxWeeksBack.value, Math.max(0, weeksBack.value + step * weeksPerPage.value));
}

const grid = computed(() => {
  const visibleWeeks = weeks.value.slice(pageStart.value, pageStart.value + weeksPerPage.value);

  const monthLabels: MonthLabel[] = [];
  for (let w = 1; w < visibleWeeks.length; w++) {
    const prev = visibleWeeks[w - 1][0].date;
    const curr = visibleWeeks[w][0].date;
    if (prev.getMonth() !== curr.getMonth()) {
      monthLabels.push({monthIdx: curr.getMonth(), weekIdx: w});
    }
  }
  const firstWeekLastDay = visibleWeeks[0].findLast((day) => day.visible);
  if (firstWeekLastDay && (!monthLabels.length || monthLabels[0].weekIdx > 2)) {
    monthLabels.unshift({monthIdx: firstWeekLastDay.date.getMonth(), weekIdx: 0});
  }

  const dayCellSize = maxWeeksBack.value > 0 || !containerWidth.value ? cellSize : (containerWidth.value - gridLeft - squareBorder) / visibleWeeks.length;
  const width = gridLeft + (dayCellSize * visibleWeeks.length) + squareBorder;
  const height = gridTop + (dayCellSize * daysInWeek);
  return {calendar: visibleWeeks, monthLabels, dayLabels, cellSize: dayCellSize, width, height};
});

const totalContributionsText = computed(() => {
  return trString(props.locale.textTotalContributions, data.value.total.toLocaleString());
});

const legendViewBox = `${cellSize} 0 ${squareSize * (colorRange.length + 2)} ${squareSize}`;

const cellInstances = new Map<Element, Instance>();
let singleton: CreateSingletonInstance | null = null;

function destroyCellTooltips() {
  for (const instance of cellInstances.values()) instance.destroy();
  cellInstances.clear();
}

watch([data, pageStart, weeksPerPage], () => { // the cells are reused for other days
  singleton!.unmount();
  singleton!.setInstances([]);
  destroyCellTooltips();
});

onMounted(() => {
  singleton = createSingleton([], {
    overrides: [],
    moveTransition: 'transform 0.1s ease-out',
    allowHTML: true,
    theme: 'tooltip',
    role: 'tooltip',
    placement: 'top',
  });
  resizeObserver = new ResizeObserver(([entry]) => {
    requestAnimationFrame(() => containerWidth.value = entry.contentRect.width); // deferred to avoid ResizeObserver loop errors
  });
  const container = elRoot.value!.parentElement!; // its width does not depend on the rendered weeks
  containerWidth.value = container.clientWidth;
  resizeObserver.observe(container);
});

onBeforeUnmount(() => {
  resizeObserver!.disconnect();
  singleton!.destroy();
  destroyCellTooltips();
});

function lazyInitTooltip(e: MouseEvent) {
  const el = e.target as Element;
  if (!singleton || cellInstances.has(el) || !el.classList.contains('heatmap-day')) return;
  cellInstances.set(el, tippy(el, {content: el.getAttribute('data-tooltip')!}));
  singleton.setInstances([...cellInstances.values()]);
}

function handleDayClick(date: Date) {
  const params = new URLSearchParams(document.location.search);
  const queryDate = params.get('date');
  // Timezone has to be stripped because toISOString() converts to UTC
  const clickedDate = new Date(date.getTime() - (date.getTimezoneOffset() * 60000)).toISOString().substring(0, 10);

  if (queryDate && queryDate === clickedDate) {
    params.delete('date');
  } else {
    params.set('date', clickedDate);
  }
  applyFilter(params, year.value);
}
</script>
<template>
  <div ref="elRoot" :class="{'is-loading': isLoading}">
    <svg class="heatmap-svg" :width="grid.width" :height="grid.height" :viewBox="`0 0 ${grid.width} ${grid.height}`">
      <g class="heatmap-month-labels" :transform="`translate(${gridLeft}, 0)`">
        <text
          v-for="m in grid.monthLabels"
          :key="m.weekIdx"
          class="heatmap-month-label"
          :x="grid.cellSize * m.weekIdx"
          :y="cellSize - squareBorder"
        >
          {{ locale.heatMapLocale.months[m.monthIdx] }}
        </text>
      </g>
      <g class="heatmap-day-labels" :transform="`translate(0, ${gridTop})`">
        <text
          v-for="day in grid.dayLabels"
          :key="day.dayIdx"
          class="heatmap-day-label"
          :x="0"
          :y="(day.rowIdx + 1) * grid.cellSize - squareBorder * 2"
        >
          {{ locale.heatMapLocale.days[day.dayIdx] }}
        </text>
      </g>
      <g class="heatmap-grid" :transform="`translate(${gridLeft}, ${gridTop})`" @mouseover="lazyInitTooltip">
        <g
          v-for="(week, w) in grid.calendar"
          :key="w"
          class="heatmap-week"
          :transform="`translate(${w * grid.cellSize}, 0)`"
        >
          <template v-for="(day, d) in week" :key="d">
            <rect
              v-if="day.visible"
              class="heatmap-day"
              :transform="`translate(0, ${d * grid.cellSize})`"
              :width="grid.cellSize - squareBorder"
              :height="grid.cellSize - squareBorder"
              :style="{fill: colorRange[day.colorIndex]}"
              :aria-label="day.ariaLabel"
              :data-tooltip="day.tooltip"
              @click="handleDayClick(day.date)"
            />
          </template>
        </g>
      </g>
    </svg>
    <div class="heatmap-footer">
      <div>{{ totalContributionsText }}</div>
      <div class="heatmap-controls">
        <div v-if="maxWeeksBack > 0" class="heatmap-pager">
          <button
            class="btn interact-bg heatmap-pager-button" type="button"
            :aria-label="locale.previousMonths" :data-tooltip-content="locale.previousMonths"
            :disabled="weeksBack === maxWeeksBack" @click="changePage(1)"
          >
            <SvgIcon name="octicon-chevron-left" :size="12"/>
          </button>
          <button
            class="btn interact-bg heatmap-pager-button" type="button"
            :aria-label="locale.nextMonths" :data-tooltip-content="locale.nextMonths"
            :disabled="weeksBack === 0" @click="changePage(-1)"
          >
            <SvgIcon name="octicon-chevron-right" :size="12"/>
          </button>
        </div>
        <span class="heatmap-year-select">
          <select class="interact-bg" :value="year" :aria-label="locale.timeRange" @change="onYearChange">
            <option v-for="optionYear in years" :key="optionYear" :value="optionYear">{{ optionYear || locale.last12Months }}</option>
          </select>
          <SvgIcon name="octicon-triangle-down" :size="12"/>
        </span>
        <div class="heatmap-legend">
          <div>{{ locale.heatMapLocale.less }}</div>
          <svg class="heatmap-legend-svg" :viewBox="legendViewBox" :height="squareSize">
            <rect
              v-for="(color, i) in colorRange"
              :key="i"
              :width="squareSize"
              :height="squareSize"
              :x="(i + 1) * cellSize"
              :style="{fill: color}"
            />
          </svg>
          <div>{{ locale.heatMapLocale.more }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
