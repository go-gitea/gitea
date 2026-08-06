<script lang="ts" setup>
import {onMounted, onUnmounted, toRaw, useTemplateRef, watch, type ShallowRef} from 'vue';
import {BarController, Chart, LineController, type ChartData, type ChartOptions} from 'chart.js';
import {chartJsColors} from '../utils/color.ts';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm'; // the package main is cjs

Chart.defaults.color = chartJsColors.text;
Chart.defaults.borderColor = chartJsColors.border;
Chart.register(BarController, LineController);

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
