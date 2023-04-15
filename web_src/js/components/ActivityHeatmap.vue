<template>
  <div id="user-heatmap">
    <!-- eslint-disable-next-line vue/no-v-html (safely generated in the backend) -->
    <div class="total-contributions" v-html="locale.contributions_in_the_last_12_months_html"/>
    <calendar-heatmap
      :locale="locale"
      :no-data-text="locale.no_contributions"
      :tooltip-formatter="(v) => tooltipFormatter(v, locale)"
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
  components: {CalendarHeatmap},
  props: {
    values: {
      type: Array,
      default: () => [],
    },
    locale: {
      type: Object,
      default: () => {},
    }
  },
  data: () => ({
    colorRange: [
      'var(--color-secondary-alpha-70)',
      'var(--color-secondary-alpha-70)',
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
    tooltipFormatter(v, locale) {
      const number = v.count.toLocaleString();
      const datetime = v.date.toISOString();
      const fallback = v.date.toLocaleDateString();
      const date = `<relative-time format="datetime" year="numeric" month="short" day="numeric" weekday="" datetime="${datetime}">${fallback}</relative-time>`;
      return locale.contributions_on.replace('%[1]s', number).replace('%[2]s', date);
    }
  },
};
</script>
