import {translateMonth, translateDay} from '../utils.ts';
import {GET} from '../modules/fetch.ts';
import {trString} from '../modules/i18n.ts';
import '../webcomponents/activity-heatmap.ts';

type HeatmapResponse = {
  heatmapData: Array<[number, number]>; // [[1617235200, 2]] = [unix timestamp, count]
  totalContributions: number;
};

export async function initHeatmap() {
  const el = document.querySelector<HTMLElement>('#user-heatmap');
  if (!el) return;

  try {
    const url = el.getAttribute('data-heatmap-url')!;
    const resp = await GET(url);
    if (!resp.ok) throw new Error(`Failed to load heatmap data: ${resp.status} ${resp.statusText}`);
    const {heatmapData, totalContributions} = await resp.json() as HeatmapResponse;

    const values = heatmapData.map(([timestamp, count]) => ({timestamp, count}));

    const totalFormatted = totalContributions.toLocaleString();
    const textTotalContributions = trString(el.getAttribute('data-locale-total-contributions')!, totalFormatted);

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
      textTotalContributions,
      noDataText: el.getAttribute('data-locale-no-contributions'),
    };

    el.setAttribute('data-values', JSON.stringify(values));
    el.setAttribute('data-locale', JSON.stringify(locale));
    el.classList.remove('is-loading');
  } catch (err) {
    console.error('Heatmap failed to load', err);
    el.textContent = 'Heatmap failed to load';
  }
}
