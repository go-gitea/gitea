<script lang="ts" setup>
import {ref, computed, watch, onBeforeUnmount, onMounted} from 'vue';
import tippy, {createSingleton} from 'tippy.js';
import type {CreateSingletonInstance, Instance} from 'tippy.js';
import {getCurrentLocale} from '../utils.ts';
import {GET} from '../modules/fetch.ts';

type HeatmapValue = {date: Date; count: number};
type HeatmapCell = {date: Date; colorIndex: number; ariaLabel: string; tooltip: string};
type MonthLabel = {monthIdx: number; weekIdx: number};

const props = defineProps<{
  heatmapUrl: string;
  userCreatedYear?: number;
  showPrivateToggle?: boolean;
  locale: {
    textTotalContributions: string;
    heatMapLocale: {months: string[]; days: string[]; on: string; more: string; less: string};
    noDataText: string;
    tooltipUnit: string;
    last12Months: string;
    showPrivate: string;
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

const params = new URLSearchParams(document.location.search);
const initialYear = parseInt(params.get('year') || '0', 10) || 0;
const initialShowPrivate = params.get('show-private') !== 'false';

const values = ref<HeatmapValue[]>([]);
const totalContributions = ref(0);
const selectedYear = ref(initialYear);
const showPrivate = ref(initialShowPrivate);
const isLoading = ref(true);

const startYear = props.userCreatedYear ? Math.min(props.userCreatedYear, currentYear) : currentYear;
const years = computed(() => {
  const list = [0]; // 0 represents "Last 12 months"
  for (let y = currentYear; y >= startYear; y--) {
    list.push(y);
  }
  return list;
});

async function fetchData() {
  isLoading.value = true;
  try {
    const url = new URL(props.heatmapUrl, window.location.origin);
    if (selectedYear.value > 0) {
      url.searchParams.set('year', String(selectedYear.value));
    }
    url.searchParams.set('show-private', String(showPrivate.value));

    const resp = await GET(url.toString());
    if (!resp.ok) throw new Error(`Failed to load heatmap data: ${resp.status} ${resp.statusText}`);
    const {heatmapData, totalContributions: total} = await resp.json();

    const heatmap: Record<string, number> = {};
    for (const [timestamp, contributions] of heatmapData) {
      // Convert to user timezone and sum contributions by date
      const dateStr = new Date(timestamp * 1000).toDateString();
      heatmap[dateStr] = (heatmap[dateStr] || 0) + contributions;
    }

    values.value = Object.entries(heatmap).map(([dateStr, count]) => {
      return {date: new Date(dateStr), count};
    });
    totalContributions.value = total;
  } catch (err) {
    console.error('Heatmap failed to load', err);
  } finally {
    isLoading.value = false;
  }
}

onMounted(() => {
  fetchData();
});

function selectYear(y: number) {
  selectedYear.value = y;
  const params = new URLSearchParams(document.location.search);
  if (y > 0) {
    params.set('year', String(y));
  } else {
    params.delete('year');
  }
  params.delete('date'); // clear date filter when changing year
  params.delete('page');

  const newSearch = params.toString();
  window.location.search = newSearch.length ? `?${newSearch}` : '';
}

watch(showPrivate, (val) => {
  const params = new URLSearchParams(document.location.search);
  params.set('show-private', String(val));
  params.delete('page');

  const newSearch = params.toString();
  window.location.search = newSearch.length ? `?${newSearch}` : '';
});

function dateKey(d: Date): string {
  return `${d.getFullYear()}${String(d.getMonth()).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`;
}

function shiftDate(d: Date, days: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + days);
  return out;
}

const limitDate = computed(() => {
  if (selectedYear.value === 0) return now;
  return new Date(selectedYear.value, 11, 31, 23, 59, 59, 999);
});

const grid = computed(() => {
  let start: Date;
  let end: Date;
  let daysCount: number;

  if (selectedYear.value === 0) {
    start = shiftDate(now, -trailingDays);
    end = now;
    daysCount = trailingDays + 1;
  } else {
    start = new Date(selectedYear.value, 0, 1);
    end = new Date(selectedYear.value, 11, 31);
    daysCount = Math.round((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24)) + 1;
  }

  const padStart = start.getDay();
  const padEnd = daysInWeek - 1 - end.getDay();
  const weekCount = (daysCount + padStart + padEnd) / daysInWeek;

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

  const width = gridLeft + (cellSize * weekCount) + squareBorder;
  const height = gridTop + (cellSize * daysInWeek);
  return {calendar, monthLabels, width, height};
});

const legendViewBox = `${cellSize} 0 ${squareSize * (colorRange.length + 2)} ${squareSize}`;

const cellInstances = new Map<Element, Instance>();
let singleton: CreateSingletonInstance | null = null;

onMounted(() => {
  singleton = createSingleton([], {
    overrides: [],
    moveTransition: 'transform 0.1s ease-out',
    allowHTML: true,
    theme: 'tooltip',
    role: 'tooltip',
    placement: 'top',
  });
});

onBeforeUnmount(() => {
  singleton?.destroy();
  for (const instance of cellInstances.values()) instance.destroy();
  cellInstances.clear();
});

watch(values, () => {
  for (const instance of cellInstances.values()) instance.destroy();
  cellInstances.clear();
  singleton?.setInstances([]);
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

const totalContributionsText = computed(() => {
  const totalFormatted = totalContributions.value.toLocaleString();
  const baseText = props.locale.textTotalContributions.replace('%s', totalFormatted);
  if (selectedYear.value === 0) {
    return baseText;
  }
  const last12MonthsPattern = new RegExp(props.locale.last12Months, 'gi');
  if (last12MonthsPattern.test(baseText)) {
    return baseText.replace(last12MonthsPattern, String(selectedYear.value));
  }
  return baseText.replace(/last 12 months/gi, String(selectedYear.value));
});
</script>

<template>
  <div class="tw-flex tw-flex-col md:tw-flex-row tw-gap-4">
    <div class="tw-flex-1 tw-min-w-0">
      <div v-if="isLoading" class="tw-flex tw-items-center tw-justify-center tw-h-32">
        <div class="ui active centered inline loader"/>
      </div>
      <div v-else>
        <svg class="heatmap-svg" :viewBox="`0 0 ${grid.width} ${grid.height}`">
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
            <text class="heatmap-day-label" :x="0" :y="20">{{ locale.heatMapLocale.days[1] }}</text>
            <text class="heatmap-day-label" :x="0" :y="44">{{ locale.heatMapLocale.days[3] }}</text>
            <text class="heatmap-day-label" :x="0" :y="69">{{ locale.heatMapLocale.days[5] }}</text>
          </g>
          <g class="heatmap-grid" :transform="`translate(${gridLeft}, ${gridTop})`" @mouseover="lazyInitTooltip">
            <g
              v-for="(week, w) in grid.calendar"
              :key="w"
              class="heatmap-week"
              :transform="`translate(${w * cellSize}, 0)`"
            >
              <template v-for="(day, d) in week" :key="d">
                <rect
                  v-if="day.date.getTime() <= limitDate.getTime()"
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
          <div>{{ totalContributionsText }}</div>
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

    <div class="heatmap-controls tw-flex tw-flex-col tw-gap-4 tw-shrink-0 tw-w-full md:tw-w-44">
      <div class="native-select tw-w-full">
        <select
          v-model="selectedYear"
          class="tw-w-full heatmap-select"
          @change="selectYear(selectedYear)"
        >
          <option
            v-for="y in years"
            :key="y"
            :value="y"
          >
            {{ y === 0 ? locale.last12Months : y }}
          </option>
        </select>
      </div>

      <div v-if="showPrivateToggle" class="ui checkbox tw-ml-2">
        <input type="checkbox" v-model="showPrivate" id="heatmap-private-toggle">
        <label for="heatmap-private-toggle" class="tw-cursor-pointer">{{ locale.showPrivate }}</label>
      </div>
    </div>
  </div>
</template>
