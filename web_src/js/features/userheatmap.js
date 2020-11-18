import Vue from 'vue';

import ActivityHeatmap from '../components/ActivityHeatmap.vue';

export default async function initUserHeatmap() {
  const el = document.getElementById('user-heatmap');
  if (!el) return;

  const values = JSON.parse(el.dataset.heatmapData).map(({contributions, timestamp}) => {
    return {date: new Date(timestamp * 1000), count: contributions};
  });

  const View = Vue.extend({
    render: (createElement) => createElement(ActivityHeatmap, {props: {values}}),
  });

  new View().$mount(el);
}
