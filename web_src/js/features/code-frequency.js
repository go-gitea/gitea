import {createApp} from 'vue';

export async function initRepoCodeFrequency() {
  const el = document.querySelector('#repo-code-frequency-chart');
  if (!el) return;

  const {default: RepoCodeFrequency} = await import(/* webpackChunkName: "code-frequency-graph" */'../components/RepoCodeFrequency.vue');
  try {
    const View = createApp(RepoCodeFrequency, {
      locale: {
        loadingTitle: el.getAttribute('data-locale-loading-title'),
        loadingTitleFailed: el.getAttribute('data-locale-loading-title-failed'),
        loadingInfo: el.getAttribute('data-locale-loading-info'),
      },
    });
    View.mount(el);
  } catch (err) {
    console.error('RepoCodeFrequency failed to load', err);
    el.textContent = el.getAttribute('data-locale-component-failed-to-load');
  }
}
