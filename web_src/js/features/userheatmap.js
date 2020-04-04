import Vue from 'vue';

import ActivityHeatmap from '../components/ActivityHeatmap.vue';

export default async function initUserHeatmap() {
  const el = document.getElementById('user-heatmap');
  if (!el) return;
  const View = Vue.extend(ActivityHeatmap);
  new View().$mount(el);
}
