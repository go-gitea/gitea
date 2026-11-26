import {createApp} from 'vue';
import DashboardRepoList from '../components/DashboardRepoList.vue';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';

const {appSubUrl} = window.config;

export function initDashboardRepoList() {
  const el = document.querySelector('#dashboard-repo-list');
  if (el) {
    createApp(DashboardRepoList).mount(el);
  }
}

export function initDashboardNavbar() {
  registerGlobalInitFunc('initDashboardNavbarLinks', (el: HTMLElement) => {
    el.querySelector('a.active')!.scrollIntoView({block: 'nearest', inline: 'nearest'}); // the links scroll horizontally on narrow screens
  });
  let savingTab = false;
  registerGlobalEventFunc('click', 'onDashboardTabClick', async (link: HTMLAnchorElement, event: MouseEvent) => {
    if (event.ctrlKey || event.metaKey || event.shiftKey) return; // let the browser open a new tab or window
    event.preventDefault();
    if (savingTab) return;
    savingTab = true;
    try {
      const resp = await POST(`${appSubUrl}/user/settings/update_preferences`, {data: {dashboardTab: link.getAttribute('data-dashboard-tab')}});
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      window.location.assign(link.href);
    } catch {
      savingTab = false;
      showErrorToast(window.config.i18n.error_occurred);
    }
  });
}
