<template>
  <div id="user-heatmap">
    <div class="total-contributions">
      {{ sum }} {{ locale.contributions_in_the_last_12_months }}
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
import {translateMonth, translateDay, getCurrentLocale} from '../utils.js';

const {i18n} = window.config;

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
      months: new Array(12).fill().map((_, idx) => translateMonth(idx)),
      days: new Array(7).fill().map((_, idx) => translateDay(idx)),
      contributions: i18n.contributions.toLocaleLowerCase(getCurrentLocale()),
      no_contributions: i18n.no_contributions,
      contributions_in_the_last_12_months: i18n.contributions_in_the_last_12_months.toLocaleLowerCase(getCurrentLocale()),
      on: i18n.on.toLocaleLowerCase(getCurrentLocale()),
      less: i18n.less,
      more: i18n.more,
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
