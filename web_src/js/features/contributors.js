import {createApp} from 'vue';
import RepoContributors from '../components/RepoContributors.vue';

export function initRepoContributors() {
  const el = document.getElementById('repo-contributors-chart');
  if (!el) return;

  try {
    const View = createApp(RepoContributors);

    View.mount(el);
  } catch (err) {
    console.error('RepoContributors failed to load', err);
    el.textContent = 'RepoContributors failed to load';
  }
}