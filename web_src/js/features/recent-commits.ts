import {createApp} from 'vue';

export async function initRepoRecentCommits() {
  const el = document.querySelector('#repo-recent-commits-chart');
  if (!el) return;

  const {default: RepoRecentCommits} = await import(/* webpackChunkName: "recent-commits-graph" */'../components/RepoRecentCommits.vue');
  try {
    const View = createApp(RepoRecentCommits, {
      locale: {
        loadingTitle: el.getAttribute('data-locale-loading-title'),
        loadingTitleFailed: el.getAttribute('data-locale-loading-title-failed'),
        loadingInfo: el.getAttribute('data-locale-loading-info'),
      },
    });
    View.mount(el);
  } catch (err) {
    console.error('RepoRecentCommits failed to load', err);
    el.textContent = el.getAttribute('data-locale-component-failed-to-load');
  }
}
