import {createApp} from 'vue';
import RepoContributors from '../components/RepoContributors.vue';

export function initRepoContributors() {
  const el = document.getElementById('repo-contributors-chart');
  if (!el) return;

  try {
    const View = createApp(RepoContributors, {
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
      }
    });
    View.mount(el);
  } catch (err) {
    console.error('RepoContributors failed to load', err);
    el.textContent = el.getAttribute('data-locale-component-failed-to-load');
  }
}
