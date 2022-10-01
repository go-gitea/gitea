import {createApp} from 'vue';
import ActivityHeatmap from '../components/ActivityHeatmap.vue';

export default function initHeatmap() {
  const el = document.getElementById('user-heatmap');
  if (!el) return;

  try {
    const heatmap = {};
    for (const {contributions, timestamp} of JSON.parse(el.getAttribute('data-heatmap-data'))) {
      // Convert to user timezone and sum contributions by date
      const dateStr = new Date(timestamp * 1000).toDateString();
      heatmap[dateStr] = (heatmap[dateStr] || 0) + contributions;
    }

    const values = Object.keys(heatmap).map((v) => {
      return {date: new Date(v), count: heatmap[v]};
    });

    const View = createApp(ActivityHeatmap, {values});

    View.mount(el);
  } catch (err) {
    console.error('Heatmap failed to load', err);
    el.textContent = 'Heatmap failed to load';
  }
}
