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
      @day-click="handleDayClick($event)"
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
  methods: {
    handleDayClick(event) {
      // Reset filter if same date is clicked
      const day = window.location.search.match(/\?date=(\d{4})-(\d{1,2})-(\d{1,2})/);
      if (day !== null) {
        if (day.length === 4) {
          if ((parseInt(day[1]) === event.date.getFullYear()) && (parseInt(day[2]) === (event.date.getMonth() + 1)) && (parseInt(day[3]) === event.date.getDate())) {
            window.location.search = '';
            return;
          }
        }
      }

      window.location.search = `?date=${event.date.getFullYear()}-${event.date.getMonth() + 1}-${event.date.getDate()}`;
    }
  },
};
</script>
<style scoped/>
