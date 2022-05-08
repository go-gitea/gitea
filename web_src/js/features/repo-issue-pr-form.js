import Vue from 'vue';
import PullRequestMergeForm from '../components/PullRequestMergeForm.vue';

export default function initPullRequestMergeForm() {
  const el = document.getElementById('pull-request-merge-form');
  if (!el) return;

  const View = Vue.extend({
    render: (createElement) => createElement(PullRequestMergeForm),
  });
  new View().$mount(el);
}
