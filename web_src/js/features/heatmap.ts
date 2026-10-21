import {createApp} from 'vue';
import {translateMonth, translateDay} from '../utils.ts';

export async function initHeatmap() {
  const el = document.querySelector<HTMLElement>('#user-heatmap');
  if (!el) return;

  try {
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
      showPrivate: el.getAttribute('data-locale-show-private')!,
      previousMonths: el.getAttribute('data-locale-previous-months')!,
      nextMonths: el.getAttribute('data-locale-next-months')!,
    };

    const {default: ActivityHeatmap} = await import('../components/ActivityHeatmap.vue');
    const View = createApp(ActivityHeatmap, {
      heatmapUrl: el.getAttribute('data-heatmap-url')!,
      userCreatedYear: parseInt(el.getAttribute('data-user-created-year') || '0') || 0,
      showPrivateToggle: el.getAttribute('data-show-private-toggle') === 'true',
      locale,
    });
    View.mount(el);
    el.classList.remove('is-loading');
  } catch (err) {
    console.error('Heatmap failed to load', err);
    el.textContent = 'Heatmap failed to load';
  }
}
