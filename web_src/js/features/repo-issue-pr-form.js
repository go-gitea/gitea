import {createApp} from 'vue';
import PullRequestMergeForm from '../components/PullRequestMergeForm.vue';

export default function initPullRequestMergeForm() {
  const el = document.getElementById('pull-request-merge-form');
  if (!el) return;

  const view = createApp(PullRequestMergeForm);
  view.mount(el);
}
