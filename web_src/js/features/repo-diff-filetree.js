import Vue from 'vue';
import PullRequestFileTree from '../components/DiffFileTree.vue';

export default function initDiffFileTree() {
  const el = document.getElementById('diff-file-tree-container');
  if (!el) return;

  const View = Vue.extend({
    render: (createElement) => createElement(PullRequestFileTree),
  });
  new View().$mount(el);
}
