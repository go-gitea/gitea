import Vue from 'vue';
import DiffFileTree from '../components/DiffFileTree.vue';
import DiffFileList from '../components/DiffFileList.vue';

export default function initDiffFileTree() {
  const el = document.getElementById('diff-file-tree-container');
  if (!el) return;

  const View = Vue.extend({
    render: (createElement) => createElement(DiffFileTree),
  });
  new View().$mount(el);

  const fileListElement = document.getElementById('diff-file-list-container');
  if (!fileListElement) return;

  const fileListView = Vue.extend({
    render: (createElement) => createElement(DiffFileList),
  });
  new fileListView().$mount(fileListElement);
}
