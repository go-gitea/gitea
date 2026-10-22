import {createApp} from 'vue';
import {translateMonth, translateDay} from '../utils.ts';
import {GET} from '../modules/fetch.ts';

type HeatmapResponse = {
  heatmapData: Array<[number, number]>; // [[1617235200, 2]] = [unix timestamp, count]
  totalContributions: number;
};

async function loadHeatmapData(heatmapUrl: string, year: number) {
  const url = new URL(heatmapUrl, window.location.origin);
  if (year) url.searchParams.set('year', String(year));
  const resp = await GET(url.href);
  if (!resp.ok) throw new Error(`Failed to load heatmap data: ${resp.status} ${resp.statusText}`);
  const {heatmapData, totalContributions} = await resp.json() as HeatmapResponse;

  const heatmap: Record<string, number> = {};
  for (const [timestamp, contributions] of heatmapData) {
    // Convert to user timezone and sum contributions by date
    const dateStr = new Date(timestamp * 1000).toDateString();
    heatmap[dateStr] = (heatmap[dateStr] || 0) + contributions;
  }
  return {
    values: Object.entries(heatmap).map(([dateStr, count]) => ({date: new Date(dateStr), count})),
    total: totalContributions,
  };
}

export async function initHeatmap() {
  const el = document.querySelector<HTMLElement>('#user-heatmap');
  if (!el) return;

  try {
    const heatmapUrl = el.getAttribute('data-heatmap-url')!;
    const yearParam = new URLSearchParams(window.location.search).get('year') ?? '';
    const urlYear = /^\d+$/.test(yearParam) ? Number(yearParam) : 0;
    const year = urlYear >= 1970 && urlYear <= 9999 ? urlYear : 0; // matches FormInt and isFeedYearValid
    const [data, {default: ActivityHeatmap}] = await Promise.all([
      loadHeatmapData(heatmapUrl, year),
      import('../components/ActivityHeatmap.vue'),
    ]);

    // last heatmap tooltip localization attempt https://github.com/go-gitea/gitea/pull/24131/commits/a83761cbbae3c2e3b4bced71e680f44432073ac8
    const locale = {
      heatMapLocale: {
        months: new Array(12).fill(undefined).map((_, idx) => translateMonth(idx)),
        days: new Array(7).fill(undefined).map((_, idx) => translateDay(idx)),
        on: ' - ', // no correct locale support for it, because in many languages the sentence is not "something on someday"
        more: el.getAttribute('data-locale-more')!,
        less: el.getAttribute('data-locale-less')!,
      },
      tooltipUnit: 'contributions',
      textTotalContributions: el.getAttribute('data-locale-total-contributions')!,
      noDataText: el.getAttribute('data-locale-no-contributions')!,
      last12Months: el.getAttribute('data-locale-last-12-months')!,
      timeRange: el.getAttribute('data-locale-time-range')!,
      previousMonths: el.getAttribute('data-locale-previous-months')!,
      nextMonths: el.getAttribute('data-locale-next-months')!,
    };

    const View = createApp(ActivityHeatmap, {
      initialYear: year,
      initialData: data,
      loadData: (newYear: number) => loadHeatmapData(heatmapUrl, newYear),
      firstYear: Number(el.getAttribute('data-user-created-year')),
      locale,
    });
    View.mount(el);
  } catch (err) {
    console.error('Heatmap failed to load', err);
    el.textContent = 'Heatmap failed to load';
  } finally {
    el.classList.remove('is-loading');
  }
}
