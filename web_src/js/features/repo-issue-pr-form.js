import {createApp} from 'vue';
import PullRequestMergeForm from '../components/PullRequestMergeForm.vue';
import * as htmx from 'htmx.org';

function initVue() {
  const el = document.getElementById('pull-request-merge-form');
  // don't call createApp if its already created
  if (!el || el.childElementCount) return;

  const view = createApp(PullRequestMergeForm);
  view.mount(el);
}

export function initRepoPullRequestMergeForm() {
  document.addEventListener('htmx:afterSettle', (ev) => {
    const tabsMenuSelector = '.page-content.repository.issue .container .menu.tabular';
    const tabsMenu = document.querySelector(tabsMenuSelector);
    const isAnchorTag = ev.detail.requestConfig.elt.tagName === 'A';
    if (tabsMenu && isAnchorTag) {
      try {
        // abort other request to prevent race-conditions
        htmx.trigger(`${tabsMenuSelector} a.is-loading`, 'htmx:abort');
      } catch {}
      tabsMenu.querySelector('a.active')?.classList.remove('active');
      ev.detail.requestConfig.elt.classList.add('active');
    }

    initVue();
  });
  initVue();
}
