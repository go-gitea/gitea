<script>
import {CalendarHeatmap} from 'vue3-calendar-heatmap';

export default {
  components: {CalendarHeatmap},
  props: {
    values: {
      type: Array,
      default: () => [],
    },
    locale: {
      type: Object,
      default: () => {},
    },
  },
  data: () => ({
    colorRange: [
      'var(--color-secondary-alpha-60)',
      'var(--color-secondary-alpha-60)',
      'var(--color-primary-light-4)',
      'var(--color-primary-light-2)',
      'var(--color-primary)',
      'var(--color-primary-dark-2)',
      'var(--color-primary-dark-4)',
    ],
    endDate: new Date(),
  }),
  mounted() {
    // work around issue with first legend color being rendered twice and legend cut off
    const legend = document.querySelector('.vch__external-legend-wrapper');
    legend.setAttribute('viewBox', '12 0 80 10');
    legend.style.marginRight = '-12px';
  },
  methods: {
    handleDayClick(e) {
      // Reset filter if same date is clicked
      const params = new URLSearchParams(document.location.search);
      const queryDate = params.get('date');
      // Timezone has to be stripped because toISOString() converts to UTC
      const clickedDate = new Date(e.date - (e.date.getTimezoneOffset() * 60000)).toISOString().substring(0, 10);

      if (queryDate && queryDate === clickedDate) {
        params.delete('date');
      } else {
        params.set('date', clickedDate);
      }

      params.delete('page');

      const newSearch = params.toString();
      window.location.search = newSearch.length ? `?${newSearch}` : '';
    },
  },
};
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
  />
</template>
<style>
/* A quick patch for vue3-calendar-heatmap's tooltip padding, to avoid conflicting with other tippy contents.
At the moment we could only identify the tooltip by its transition property.
https://github.com/razorness/vue3-calendar-heatmap/blob/955626176cb5dc3d3ead8120475c2e5e753cc392/src/components/CalendarHeatmap.vue#L202
This selector should be replaced by a more specific one if the library adds a CSS class.
*/
[data-tippy-root][style*="transition: transform 0.1s ease-out"] .tippy-box .tippy-content {
  transition: none !important;
  padding: 0.5rem 1rem;
  background-color: var(--color-tooltip-bg);
  color: var(--color-tooltip-text);
  border: none;
  border-radius: var(--border-radius);
}
</style>
