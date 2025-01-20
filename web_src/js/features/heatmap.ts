import {createApp} from 'vue';
import ActivityHeatmap from '../components/ActivityHeatmap.vue';
import {translateMonth, translateDay} from '../utils.ts';

export function initHeatmap() {
  const el = document.querySelector('#user-heatmap');
  if (!el) return;

  try {
    const heatmap: Record<string, number> = {};
    for (const {contributions, timestamp} of JSON.parse(el.getAttribute('data-heatmap-data'))) {
      // Convert to user timezone and sum contributions by date
      const dateStr = new Date(timestamp * 1000).toDateString();
      heatmap[dateStr] = (heatmap[dateStr] || 0) + contributions;
    }

    const values = Object.keys(heatmap).map((v) => {
      return {date: new Date(v), count: heatmap[v]};
    });

    // last heatmap tooltip localization attempt https://github.com/go-gitea/gitea/pull/24131/commits/a83761cbbae3c2e3b4bced71e680f44432073ac8
    const locale = {
      heatMapLocale: {
        months: new Array(12).fill(undefined).map((_, idx) => translateMonth(idx)),
        days: new Array(7).fill(undefined).map((_, idx) => translateDay(idx)),
        on: ' - ', // no correct locale support for it, because in many languages the sentence is not "something on someday"
        more: el.getAttribute('data-locale-more'),
        less: el.getAttribute('data-locale-less'),
      },
      tooltipUnit: 'contributions',
      textTotalContributions: el.getAttribute('data-locale-total-contributions'),
      noDataText: el.getAttribute('data-locale-no-contributions'),
    };

    const View = createApp(ActivityHeatmap, {values, locale});
    View.mount(el);
    el.classList.remove('is-loading');
  } catch (err) {
    console.error('Heatmap failed to load', err);
    el.textContent = 'Heatmap failed to load';
  }
}
