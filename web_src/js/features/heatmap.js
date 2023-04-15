import {createApp} from 'vue';
import ActivityHeatmap from '../components/ActivityHeatmap.vue';
import {translateMonth, translateDay} from '../utils.js';

export function initHeatmap() {
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

    const locale = {
      months: new Array(12).fill().map((_, idx) => translateMonth(idx)),
      days: new Array(7).fill().map((_, idx) => translateDay(idx)),
      contributions_in_the_last_12_months_html: el.getAttribute('data-locale-total-contributions-html'),
      no_contributions: el.getAttribute('data-locale-no-contributions'),
      more: el.getAttribute('data-locale-more'),
      less: el.getAttribute('data-locale-less'),
      contributions_on_n: el.getAttribute('data-locale-contributions-on-n'),
      contributions_on_1: el.getAttribute('data-locale-contributions-on-1'),
    };

    const View = createApp(ActivityHeatmap, {values, locale});

    View.mount(el);
  } catch (err) {
    console.error('Heatmap failed to load', err);
    el.textContent = 'Heatmap failed to load';
  }
}
