import {createApp} from 'vue';
import DashboardRepoList from '../components/DashboardRepoList.vue';

export function initDashboardRepoList() {
  const el = document.querySelector('#dashboard-repo-list');
  if (el) {
    el.classList.remove('is-loading');
    createApp(DashboardRepoList).mount(el);
  }
}
