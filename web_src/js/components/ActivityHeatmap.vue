<script lang="ts" setup>
// TODO: Switch to upstream after https://github.com/razorness/vue3-calendar-heatmap/pull/34 is merged
import {CalendarHeatmap} from '@silverwind/vue3-calendar-heatmap';
import {onMounted, ref} from 'vue';
import type {Value as HeatmapValue, Locale as HeatmapLocale} from '@silverwind/vue3-calendar-heatmap';

defineProps<{
  values?: HeatmapValue[];
  locale: {
    textTotalContributions: string;
    heatMapLocale: Partial<HeatmapLocale>;
    noDataText: string;
    tooltipUnit: string;
  };
}>();

const colorRange = [
  'var(--color-secondary-alpha-60)',
  'var(--color-secondary-alpha-60)',
  'var(--color-primary-light-4)',
  'var(--color-primary-light-2)',
  'var(--color-primary)',
  'var(--color-primary-dark-2)',
  'var(--color-primary-dark-4)',
];

const endDate = ref(new Date());

onMounted(() => {
  // work around issue with first legend color being rendered twice and legend cut off
  const legend = document.querySelector<HTMLElement>('.vch__external-legend-wrapper');
  legend.setAttribute('viewBox', '12 0 80 10');
  legend.style.marginRight = '-12px';
});

function handleDayClick(e: Event & {date: Date}) {
  // Reset filter if same date is clicked
  const params = new URLSearchParams(document.location.search);
  const queryDate = params.get('date');
  // Timezone has to be stripped because toISOString() converts to UTC
  const clickedDate = new Date(e.date.getTime() - (e.date.getTimezoneOffset() * 60000)).toISOString().substring(0, 10);

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
  <div class="total-contributions">
    {{ locale.textTotalContributions }}
  </div>
  <calendar-heatmap
    :locale="locale.heatMapLocale"
    :no-data-text="locale.noDataText"
    :tooltip-unit="locale.tooltipUnit"
    :end-date="endDate"
    :values="values"
    :range-color="colorRange"
    @day-click="handleDayClick($event)"
    :tippy-props="{theme: 'tooltip'}"
  />
</template>
