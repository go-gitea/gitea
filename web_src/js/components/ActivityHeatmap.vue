<template>
  <div id="user-heatmap">
    <div class="total-contributions">
      {{ values.length }} contributions in the last 12 months
    </div>
    <calendar-heatmap
      :locale="locale"
      :no-data-text="locale.no_contributions"
      :tooltip-unit="locale.contributions"
      :end-date="endDate"
      :values="values"
      :range-color="colorRange"
    />
  </div>
</template>
<script>
import {CalendarHeatmap} from 'vue-calendar-heatmap';

export default {
  name: 'ActivityHeatmap',
  components: {CalendarHeatmap},
  props: {
    values: {
      type: Array,
      default: () => [],
    },
  },
  data: () => ({
    colorRange: [
      'var(--color-secondary-alpha-70)',
      'var(--color-primary-light-4)',
      'var(--color-primary-light-2)',
      'var(--color-primary)',
      'var(--color-primary-dark-2)',
      'var(--color-primary-dark-4)',
    ],
    endDate: new Date(),
    locale: {
      contributions: 'contributions',
      no_contributions: 'No contributions',
    },
  }),
};
</script>
<style scoped/>
