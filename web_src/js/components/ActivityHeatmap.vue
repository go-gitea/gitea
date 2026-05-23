<script lang="ts" setup>
import {computed, onBeforeUnmount, onMounted} from 'vue';
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

const squareSize = 10;
const squareBorder = 2;
const cellSize = squareSize + squareBorder;
const daysInWeek = 7;
const trailingDays = 365;
const gridLeft = Math.ceil(squareSize * 2.5);
const gridTop = squareSize + squareSize / 2;

const now = new Date();

function dateKey(d: Date): string {
  return `${d.getFullYear()}${String(d.getMonth()).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`;
}

function shiftDate(d: Date, days: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + days);
  return out;
}

const grid = computed(() => {
  const start = shiftDate(now, -trailingDays);
  const padStart = start.getDay();
  const padEnd = daysInWeek - 1 - now.getDay();
  const weekCount = (trailingDays + 1 + padStart + padEnd) / daysInWeek;

  const maxCount = props.values.length ? Math.max(...props.values.map((v) => v.count)) : 0;
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
    for (let d = 0; d < daysInWeek; d++) {
      const hit = activities.get(dateKey(cursor));
      const dateStr = `${months[cursor.getMonth()]} ${cursor.getDate()}, ${cursor.getFullYear()}`;
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
  <div>
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
              v-if="day.date < now"
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
      <div>{{ locale.textTotalContributions }}</div>
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
</template>
