<template>
  <div class="heatmap-container">
    <div v-show="isLoading">
      <slot name="loading"/>
    </div>
    <div v-if="!isLoading" class="total-contributions">
      {{ values.length }} contributions in the last 12 months
    </div>
    <calendar-heatmap
      v-show="!isLoading"
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
const {AppSubUrl, heatmapUser} = window.config;

export default {
  name: 'ActivityHeatmap',
  components: {CalendarHeatmap},
  data: () => ({
    isLoading: true,
    colorRange: [
      'var(--color-secondary-alpha-70)',
      'var(--color-primary-alpha-60)',
      'var(--color-primary-alpha-70)',
      'var(--color-primary-alpha-80)',
      'var(--color-primary-alpha-90)',
      'var(--color-primary)',
    ],
    endDate: new Date(),
    values: [],
    locale: {
      contributions: 'contributions',
      no_contributions: 'No contributions',
    },
  }),
  async mounted() {
    const res = await fetch(`${AppSubUrl}/api/v1/users/${heatmapUser}/heatmap`);
    const data = await res.json();
    this.values = data.map(({contributions, timestamp}) => {
      return {date: new Date(timestamp * 1000), count: contributions};
    });
    this.isLoading = false;
  },
};
</script>
<style scoped/>
