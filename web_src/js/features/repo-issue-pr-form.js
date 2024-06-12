import {createApp} from 'vue';
import PullRequestMergeForm from '../components/PullRequestMergeForm.vue';

export function initRepoPullRequestMergeForm() {
  const el = document.querySelector('#pull-request-merge-form');
  if (!el) return;

  const view = createApp(PullRequestMergeForm);
  view.mount(el);
}
