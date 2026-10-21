<script lang="ts" setup>
import {computed, onBeforeUnmount, onMounted, ref, useTemplateRef, watch} from 'vue';
import tippy, {createSingleton} from 'tippy.js';
import type {CreateSingletonInstance, Instance} from 'tippy.js';
import {getCurrentLocale} from '../utils.ts';
import {html} from '../utils/html.ts';
import {GET} from '../modules/fetch.ts';
import {trString} from '../modules/i18n.ts';
import SvgIcon from './SvgIcon.vue';

type HeatmapValue = {date: Date; count: number};
type HeatmapCell = {date: Date; visible: boolean; colorIndex: number; ariaLabel: string; tooltip: string};
type MonthLabel = {monthIdx: number; weekIdx: number};
type DayLabel = {dayIdx: number; rowIdx: number};
type HeatmapResponse = {
  heatmapData: Array<[number, number]>; // [[1617235200, 2]] = [unix timestamp, count]
  totalContributions: number;
};

const props = defineProps<{
  heatmapUrl: string;
  userCreatedYear: number;
  showPrivateToggle: boolean;
  locale: {
    textTotalContributions: string;
    heatMapLocale: {months: string[]; days: string[]; on: string; more: string; less: string};
    noDataText: string;
    tooltipUnit: string;
    last12Months: string;
    showPrivate: string;
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

const urlParams = new URLSearchParams(window.location.search);
const initialYear = parseInt(urlParams.get('year') || '0') || 0; // 0 means the last 12 months
const selectedYear = ref(initialYear);
const showPrivate = ref(urlParams.get('show-private') !== 'false');
const years = [0];
const firstYear = Math.min(props.userCreatedYear || currentYear, currentYear, initialYear || currentYear);
for (let y = currentYear; y >= firstYear; y--) years.push(y);

const values = ref<HeatmapValue[]>([]);
const totalContributions = ref(0);
const isLoading = ref(true);

const elRoot = useTemplateRef('elRoot');
const containerWidth = ref(0);
let resizeObserver: ResizeObserver | null = null;

async function loadData() {
  try {
    const url = new URL(props.heatmapUrl, window.location.origin);
    if (selectedYear.value) url.searchParams.set('year', String(selectedYear.value));
    if (!showPrivate.value) url.searchParams.set('show-private', 'false');
    const resp = await GET(url.href);
    if (!resp.ok) throw new Error(`Failed to load heatmap data: ${resp.status} ${resp.statusText}`);
    const {heatmapData, totalContributions: total} = await resp.json() as HeatmapResponse;

    const heatmap: Record<string, number> = {};
    for (const [timestamp, contributions] of heatmapData) {
      // Convert to user timezone and sum contributions by date
      const dateStr = new Date(timestamp * 1000).toDateString();
      heatmap[dateStr] = (heatmap[dateStr] || 0) + contributions;
    }
    values.value = Object.entries(heatmap).map(([dateStr, count]) => ({date: new Date(dateStr), count}));
    totalContributions.value = total;
  } catch (err) {
    console.error('Heatmap failed to load', err);
  } finally {
    isLoading.value = false;
  }
}

// the year and the private toggle also filter the activity feed below, so the page is reloaded
function reloadWithParam(key: string, value: string | null) {
  const params = new URLSearchParams(window.location.search);
  if (value === null) {
    params.delete(key);
  } else {
    params.set(key, value);
  }
  params.delete('date');
  params.delete('page');
  const newSearch = params.toString();
  window.location.search = newSearch.length ? `?${newSearch}` : '';
}

function onYearChange() {
  reloadWithParam('year', selectedYear.value ? String(selectedYear.value) : null);
}

function onShowPrivateChange() {
  reloadWithParam('show-private', showPrivate.value ? null : 'false');
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

const calendar = computed(() => {
  const year = selectedYear.value;
  const start = year ? new Date(year, 0, 1) : shiftDate(now, -trailingDays);
  const end = year ? new Date(year, 11, 31) : now;
  const days = Math.round((new Date(end.getFullYear(), end.getMonth(), end.getDate()).getTime() -
    new Date(start.getFullYear(), start.getMonth(), start.getDate()).getTime()) / 86400000) + 1;
  const padStart = (start.getDay() - firstDayIdx + daysInWeek) % daysInWeek;
  const padEnd = (firstDayIdx - end.getDay() - 1 + daysInWeek) % daysInWeek;
  const weekCount = Math.round((days + padStart + padEnd) / daysInWeek);

  const maxCount = values.value.length ? Math.max(...values.value.map((v) => v.count)) : 0;
  const max = maxCount > 0 ? Math.ceil(maxCount / 5 * 4) : 1;

  const activities = new Map<string, {count: number; colorIndex: number}>();
  for (const {date, count} of values.value) {
    const colorIndex = count >= max ? 4 : Math.max(1, Math.ceil((count / max) * 3));
    activities.set(dateKey(date), {count, colorIndex});
  }

  const {on} = props.locale.heatMapLocale;
  const {noDataText, tooltipUnit} = props.locale;
  const currentLocale = getCurrentLocale();

  const cursorStart = shiftDate(start, -padStart);
  const cursor = new Date(cursorStart.getFullYear(), cursorStart.getMonth(), cursorStart.getDate());
  const weeks: HeatmapCell[][] = [];
  for (let w = 0; w < weekCount; w++) {
    const week: HeatmapCell[] = [];
    for (let d = 0; d < daysInWeek; d++) {
      const hit = activities.get(dateKey(cursor));
      const dateStr = cursor.toLocaleDateString(currentLocale, {year: 'numeric', month: 'short', day: 'numeric'});
      const head = hit ? `${hit.count} ${tooltipUnit}` : noDataText;
      week.push({
        date: new Date(cursor),
        // a year only shows its own days, the last 12 months fill the first week like before
        visible: cursor < now && (!year || cursor.getFullYear() === year),
        colorIndex: hit ? hit.colorIndex : 0,
        ariaLabel: `${head} ${on} ${dateStr}`,
        tooltip: html`<b>${head}</b> ${on} ${dateStr}`,
      });
      cursor.setDate(cursor.getDate() + 1);
    }
    weeks.push(week);
  }
  return weeks;
});

// cells and fonts keep their size, a viewport too narrow for the whole range pages through equally sized windows
// of weeks instead, e.g. two halves of about 6 months on a phone
const weeksPerPage = computed(() => {
  const weekCount = calendar.value.length;
  const fitting = Math.floor((containerWidth.value - gridLeft - squareBorder) / cellSize);
  if (!containerWidth.value || fitting >= weekCount) return weekCount;
  return Math.ceil(weekCount / Math.ceil(weekCount / Math.max(1, fitting)));
});
const pageEnd = ref(0); // exclusive week index, the window starts with the most recent weeks
watch([() => calendar.value.length, weeksPerPage], () => {
  pageEnd.value = calendar.value.length;
}, {immediate: true});
const isPaged = computed(() => weeksPerPage.value < calendar.value.length);
const pageStart = computed(() => Math.max(0, pageEnd.value - weeksPerPage.value));

function showPreviousWeeks() {
  pageEnd.value = Math.max(weeksPerPage.value, pageEnd.value - weeksPerPage.value);
}

function showNextWeeks() {
  pageEnd.value = Math.min(calendar.value.length, pageEnd.value + weeksPerPage.value);
}

const grid = computed(() => {
  const visibleWeeks = calendar.value.slice(pageStart.value, pageEnd.value);

  const monthLabels: MonthLabel[] = [];
  for (let w = 1; w < visibleWeeks.length; w++) {
    const prev = visibleWeeks[w - 1][0].date;
    const curr = visibleWeeks[w][0].date;
    if (prev.getMonth() !== curr.getMonth()) {
      monthLabels.push({monthIdx: curr.getMonth(), weekIdx: w});
    }
  }
  // a window starting mid-month still names its first month, if there is room before the next label
  if (isPaged.value && visibleWeeks.length && (!monthLabels.length || monthLabels[0].weekIdx > 2)) {
    monthLabels.unshift({monthIdx: visibleWeeks[0][daysInWeek - 1].date.getMonth(), weekIdx: 0});
  }

  const dayLabels: DayLabel[] = [];
  for (let i = 0; i < daysInWeek; i++) {
    const labelDay = firstDayIdx + i;
    if (labelDay % 2 === 0) continue; // only show "Mon/Wed/Fri/Sun" because of limited vertical space
    const dayIdx = labelDay % daysInWeek;
    dayLabels.push({dayIdx, rowIdx: i});
  }

  const width = gridLeft + (cellSize * visibleWeeks.length) + squareBorder;
  const height = gridTop + (cellSize * daysInWeek);
  return {calendar: visibleWeeks, monthLabels, dayLabels, width, height};
});

const totalContributionsText = computed(() => {
  return trString(props.locale.textTotalContributions, totalContributions.value.toLocaleString());
});

const legendViewBox = `${cellSize} 0 ${squareSize * (colorRange.length + 2)} ${squareSize}`;

const cellInstances = new Map<Element, Instance>();
let singleton: CreateSingletonInstance | null = null;

function destroyTooltips() {
  for (const instance of cellInstances.values()) instance.destroy();
  cellInstances.clear();
  singleton?.setInstances([]);
}

// the cells are re-rendered on paging, so their tooltips must not outlive them
watch(pageStart, destroyTooltips);

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
    containerWidth.value = entry.contentRect.width;
  });
  resizeObserver.observe(elRoot.value!);
  loadData();
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  destroyTooltips();
  singleton?.destroy();
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

  params.delete('page');

  const newSearch = params.toString();
  window.location.search = newSearch.length ? `?${newSearch}` : '';
}
</script>
<template>
  <div ref="elRoot">
    <div v-if="isPaged" class="heatmap-pager">
      <button
        class="heatmap-pager-button" type="button"
        :aria-label="locale.previousMonths" :data-tooltip-content="locale.previousMonths"
        :disabled="pageStart === 0" @click="showPreviousWeeks"
      >
        <SvgIcon name="octicon-chevron-left" :size="12"/>
      </button>
      <button
        class="heatmap-pager-button" type="button"
        :aria-label="locale.nextMonths" :data-tooltip-content="locale.nextMonths"
        :disabled="pageEnd === calendar.length" @click="showNextWeeks"
      >
        <SvgIcon name="octicon-chevron-right" :size="12"/>
      </button>
    </div>
    <svg class="heatmap-svg" :width="grid.width" :height="grid.height" :viewBox="`0 0 ${grid.width} ${grid.height}`">
      <g class="heatmap-month-labels" :transform="`translate(${gridLeft}, 0)`">
        <text
          v-for="m in grid.monthLabels"
          :key="m.weekIdx"
          class="heatmap-month-label"
          :x="cellSize * m.weekIdx"
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
          :y="day.rowIdx * cellSize + squareSize - squareBorder"
        >
          {{ locale.heatMapLocale.days[day.dayIdx] }}
        </text>
      </g>
      <g v-if="!isLoading" class="heatmap-grid" :transform="`translate(${gridLeft}, ${gridTop})`" @mouseover="lazyInitTooltip">
        <g
          v-for="(week, w) in grid.calendar"
          :key="w"
          class="heatmap-week"
          :transform="`translate(${w * cellSize}, 0)`"
        >
          <template v-for="(day, d) in week" :key="d">
            <rect
              v-if="day.visible"
              class="heatmap-day"
              :transform="`translate(0, ${d * cellSize})`"
              :width="squareSize"
              :height="squareSize"
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
      <div>{{ isLoading ? '' : totalContributionsText }}</div>
      <div class="heatmap-controls">
        <label v-if="showPrivateToggle" class="heatmap-private-toggle">
          <input v-model="showPrivate" type="checkbox" @change="onShowPrivateChange">
          {{ locale.showPrivate }}
        </label>
        <select v-model="selectedYear" class="heatmap-year-select" @change="onYearChange">
          <option v-for="y in years" :key="y" :value="y">{{ y || locale.last12Months }}</option>
        </select>
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
