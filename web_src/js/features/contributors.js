import {createApp} from 'vue';

export async function initRepoContributors() {
  const el = document.querySelector('#repo-contributors-chart');
  if (!el) return;

  const {default: RepoContributors} = await import(/* webpackChunkName: "contributors-graph" */'../components/RepoContributors.vue');
  try {
    const View = createApp(RepoContributors, {
      repoLink: el.getAttribute('data-repo-link'),
      locale: {
        filterLabel: el.getAttribute('data-locale-filter-label'),
        contributionType: {
          commits: el.getAttribute('data-locale-contribution-type-commits'),
          additions: el.getAttribute('data-locale-contribution-type-additions'),
          deletions: el.getAttribute('data-locale-contribution-type-deletions'),
        },

        loadingTitle: el.getAttribute('data-locale-loading-title'),
        loadingTitleFailed: el.getAttribute('data-locale-loading-title-failed'),
        loadingInfo: el.getAttribute('data-locale-loading-info'),
      },
    });
    View.mount(el);
  } catch (err) {
    console.error('RepoContributors failed to load', err);
    el.textContent = el.getAttribute('data-locale-component-failed-to-load');
  }
}
