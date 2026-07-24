import {createApp} from 'vue';
import {translateMonth, translateDay} from '../utils.ts';

export async function initHeatmap() {
  const el = document.querySelector<HTMLElement>('#user-heatmap');
  if (!el) return;

  try {
    const heatmapUrl = el.getAttribute('data-heatmap-url')!;
    const userCreatedYear = parseInt(el.getAttribute('data-user-created-year') || '0', 10) || 0;
    const showPrivateToggle = el.getAttribute('data-show-private-toggle') === 'true';

    const locale = {
      heatMapLocale: {
        months: new Array(12).fill(undefined).map((_, idx) => translateMonth(idx)),
        days: new Array(7).fill(undefined).map((_, idx) => translateDay(idx)),
        on: ' - ',
        more: el.getAttribute('data-locale-more'),
        less: el.getAttribute('data-locale-less'),
      },
      tooltipUnit: 'contributions',
      textTotalContributions: el.getAttribute('data-locale-total-contributions'),
      noDataText: el.getAttribute('data-locale-no-contributions'),
      last12Months: el.getAttribute('data-locale-last-12-months'),
      showPrivate: el.getAttribute('data-locale-show-private'),
    };

    const {default: ActivityHeatmap} = await import('../components/ActivityHeatmap.vue');
    const View = createApp(ActivityHeatmap, {
      heatmapUrl,
      userCreatedYear,
      showPrivateToggle,
      locale,
    });
    View.mount(el);
    el.classList.remove('is-loading');
  } catch (err) {
    console.error('Heatmap failed to initialize', err);
    el.textContent = 'Heatmap failed to initialize';
  }
}
