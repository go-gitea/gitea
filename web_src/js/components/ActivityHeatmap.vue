<script lang="ts" setup>
import {computed, onBeforeUnmount, onMounted, shallowRef} from 'vue';
import tippy, {createSingleton} from 'tippy.js';
import type {CreateSingletonInstance, Instance} from 'tippy.js';

type HeatmapValue = {date: Date; count: number};
type HeatmapCell = {date: Date; colorIndex: number; ariaLabel: string; tooltip: string};
type MonthLabel = {monthIdx: number; weekIdx: number};

const props = defineProps<{
  values: HeatmapValue[];
  locale: {
    textTotalContributions: string;
    heatMapLocale: {months: string[]; days: string[]; on: string; more: string; less: string};
    noDataText: string;
    tooltipUnit: string;
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

const SQUARE_SIZE = 10;
const SQUARE_BORDER = 2;
const CELL = SQUARE_SIZE + SQUARE_BORDER;
const DAYS_IN_WEEK = 7;
const TRAILING_DAYS = 365;
const LEFT = Math.ceil(SQUARE_SIZE * 2.5);
const TOP = SQUARE_SIZE + SQUARE_SIZE / 2;

const now = shallowRef(new Date());

function dateKey(d: Date): string {
  return `${d.getFullYear()}${String(d.getMonth()).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`;
}

function shiftDate(d: Date, days: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + days);
  return out;
}

const heatmap = computed(() => {
  const end = now.value;
  const start = shiftDate(end, -TRAILING_DAYS);
  const padStart = start.getDay();
  const padEnd = DAYS_IN_WEEK - 1 - end.getDay();
  const weekCount = (TRAILING_DAYS + 1 + padStart + padEnd) / DAYS_IN_WEEK;

  const counts = props.values.map((v) => v.count);
  const maxCount = counts.length ? Math.max(...counts) : 0;
  const max = maxCount > 0 ? Math.ceil(maxCount / 5 * 4) : 1;

  const activities = new Map<string, {count: number; colorIndex: number}>();
  for (const {date, count} of props.values) {
    const colorIndex = count >= max ? 4 : Math.max(1, Math.ceil((count / max) * 3));
    activities.set(dateKey(date), {count, colorIndex});
  }

  const {months, on} = props.locale.heatMapLocale;
  const {noDataText, tooltipUnit} = props.locale;

  const cursorStart = shiftDate(start, -padStart);
  const cursor = new Date(cursorStart.getFullYear(), cursorStart.getMonth(), cursorStart.getDate());
  const calendar: HeatmapCell[][] = [];
  for (let w = 0; w < weekCount; w++) {
    const week: HeatmapCell[] = [];
    for (let d = 0; d < DAYS_IN_WEEK; d++) {
      const hit = activities.get(dateKey(cursor));
      const dateStr = `${months[cursor.getMonth()]} ${cursor.getDate()}, ${cursor.getFullYear()}`;
      const head = hit ? `${hit.count} ${tooltipUnit}` : noDataText;
      week.push({
        date: new Date(cursor.valueOf()),
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
    if (prev.getFullYear() < curr.getFullYear() || prev.getMonth() < curr.getMonth()) {
      monthLabels.push({monthIdx: curr.getMonth(), weekIdx: w});
    }
  }

  const width = LEFT + (CELL * weekCount) + SQUARE_BORDER;
  const height = TOP + (CELL * DAYS_IN_WEEK);
  return {calendar, monthLabels, width, height};
});

const viewbox = computed(() => `0 0 ${heatmap.value.width} ${heatmap.value.height}`);
const legendViewbox = `${CELL} 0 ${SQUARE_SIZE * (colorRange.length + 2)} ${SQUARE_SIZE}`;

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

function lazyInitTooltip(e: MouseEvent) {
  const el = e.target as Element;
  if (!singleton || cellInstances.has(el) || !el.classList?.contains('vch__day__square')) return;
  const content = el.getAttribute('data-tooltip') ?? '';
  cellInstances.set(el, tippy(el, {content}));
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
  <div class="vch__container">
    <svg class="vch__wrapper" :viewBox="viewbox">
      <g class="vch__months__labels__wrapper" :transform="`translate(${LEFT}, 0)`">
        <text
          v-for="m in heatmap.monthLabels"
          :key="m.weekIdx"
          class="vch__month__label"
          :x="CELL * m.weekIdx"
          :y="CELL - SQUARE_BORDER"
        >
          {{ locale.heatMapLocale.months[m.monthIdx] }}
        </text>
      </g>
      <g class="vch__days__labels__wrapper" :transform="`translate(0, ${TOP})`">
        <text class="vch__day__label" :x="0" :y="20">{{ locale.heatMapLocale.days[1] }}</text>
        <text class="vch__day__label" :x="0" :y="44">{{ locale.heatMapLocale.days[3] }}</text>
        <text class="vch__day__label" :x="0" :y="69">{{ locale.heatMapLocale.days[5] }}</text>
      </g>
      <g class="vch__year__wrapper" :transform="`translate(${LEFT}, ${TOP})`" @mouseover="lazyInitTooltip">
        <g
          v-for="(week, w) in heatmap.calendar"
          :key="w"
          class="vch__month__wrapper"
          :transform="`translate(${w * CELL}, 0)`"
        >
          <template v-for="(day, d) in week" :key="d">
            <rect
              v-if="day.date < now"
              class="vch__day__square"
              :transform="`translate(0, ${d * CELL})`"
              :width="SQUARE_SIZE"
              :height="SQUARE_SIZE"
              :style="{fill: colorRange[day.colorIndex]}"
              :aria-label="day.ariaLabel"
              :data-tooltip="day.tooltip"
              @click="handleDayClick(day.date)"
            />
          </template>
        </g>
      </g>
    </svg>
    <div class="vch__legend">
      <div class="vch__legend-left">{{ locale.textTotalContributions }}</div>
      <div class="vch__legend-right">
        <div class="vch__legend">
          <div>{{ locale.heatMapLocale.less }}</div>
          <svg
            class="vch__external-legend-wrapper"
            :viewBox="legendViewbox"
            :height="SQUARE_SIZE"
          >
            <g class="vch__legend__wrapper">
              <rect
                v-for="(color, i) in colorRange"
                :key="i"
                :width="SQUARE_SIZE"
                :height="SQUARE_SIZE"
                :x="(i + 1) * CELL"
                :style="{fill: color}"
              />
            </g>
          </svg>
          <div>{{ locale.heatMapLocale.more }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
