<script lang="ts" setup>
import {onMounted, onUnmounted, toRaw, useTemplateRef, watch, type ShallowRef} from 'vue';
import {_adapters, BarController, Chart, LinearScale, LineController, TimeScale, type ChartData, type ChartOptions, type TimeUnit} from 'chart.js';
import {chartJsColors} from '../utils/color.ts';
import dayjs from 'dayjs';
import advancedFormat from 'dayjs/plugin/advancedFormat.js';
import quarterOfYear from 'dayjs/plugin/quarterOfYear.js';
import isoWeek from 'dayjs/plugin/isoWeek.js';
import type {ConfigType, ManipulateType} from 'dayjs';

dayjs.extend(advancedFormat); // the quarter format needs the `Q` token
dayjs.extend(quarterOfYear);
dayjs.extend(isoWeek);

Chart.defaults.color = chartJsColors.text;
Chart.defaults.borderColor = chartJsColors.border;
Chart.register(BarController, LineController, LinearScale, TimeScale);

// minimal port of chartjs-adapter-dayjs-4, MIT license, Copyright (c) 2022 bolstycjw
_adapters._date.override({
  formats: () => ({
    datetime: 'MMM D, YYYY, h:mm:ss a',
    millisecond: 'h:mm:ss.SSS a',
    second: 'h:mm:ss a',
    minute: 'h:mm a',
    hour: 'hA',
    day: 'MMM D',
    week: 'MMM D, YYYY',
    month: 'MMM YYYY',
    quarter: '[Q]Q - YYYY',
    year: 'YYYY',
  }),
  parse: (value: ConfigType) => {
    const date = dayjs(value);
    return date.isValid() ? date.valueOf() : null;
  },
  format: (time: number, format: string) => dayjs(time).format(format),
  add: (time: number, amount: number, unit: TimeUnit) => dayjs(time).add(amount, unit as ManipulateType).valueOf(), // the quarter plugin widens this at runtime
  diff: (max: number, min: number, unit: TimeUnit) => dayjs(max).diff(min, unit),
  startOf: (time: number, unit: TimeUnit | 'isoWeek', weekday?: number | boolean) => {
    // chart.js passes `true` here when `time.isoWeekday` is enabled without a day
    if (unit === 'isoWeek') {
      const day = typeof weekday === 'number' && weekday > 0 && weekday < 7 ? weekday : 1;
      return dayjs(time).isoWeekday(day).startOf('day').valueOf();
    }
    return dayjs(time).startOf(unit).valueOf();
  },
  endOf: (time: number, unit: TimeUnit) => dayjs(time).endOf(unit).valueOf(),
});

const props = defineProps<{
  type: 'bar' | 'line',
  data: ChartData,
  options: ChartOptions,
}>();

const elCanvas = useTemplateRef('elCanvas') as Readonly<ShallowRef<HTMLCanvasElement>>;
let chart: Chart | undefined;

// chart.js mutates what it gets, so it must never see a reactive proxy
onMounted(() => {
  chart = new Chart(elCanvas.value, {type: props.type, data: toRaw(props.data), options: toRaw(props.options)});
});

onUnmounted(() => {
  chart?.destroy();
});

watch([() => props.data, () => props.options], ([data, options]) => {
  if (!chart) return; // chart creation failed
  chart.data = toRaw(data);
  chart.options = toRaw(options);
  chart.update();
});
</script>

<template>
  <canvas ref="elCanvas" role="img"/>
</template>
