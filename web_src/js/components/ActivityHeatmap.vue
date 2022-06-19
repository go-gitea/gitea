<template>
  <div id="user-heatmap">
    <div class="total-contributions">
      {{ sum }} contributions in the last 12 months
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
import {CalendarHeatmap} from 'vue3-calendar-heatmap';

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
  computed: {
    sum() {
      let s = 0;
      for (let i = 0; i < this.values.length; i++) {
        s += this.values[i].count;
      }
      return s;
    }
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

      const newSearch = params.toString();
      window.location.search = newSearch.length ? `?${newSearch}` : '';
    }
  },
};
</script>
