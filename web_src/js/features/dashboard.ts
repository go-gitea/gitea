import {createApp} from 'vue';
import DashboardRepoList from '../components/DashboardRepoList.vue';

export function initDashboardRepoList() {
  const el = document.querySelector('#dashboard-repo-list');
  if (el) {
    createApp(DashboardRepoList).mount(el);
  }
}
