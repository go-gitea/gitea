import {createApp} from 'vue';
import DiffFileTree from '../components/DiffFileTree.vue';
import DiffFileList from '../components/DiffFileList.vue';

export function initDiffFileTree() {
  const el = document.querySelector('#diff-file-tree');
  if (!el) return;

  const fileTreeView = createApp(DiffFileTree);
  fileTreeView.mount(el);

  const fileListElement = document.querySelector('#diff-file-list');
  if (!fileListElement) return;

  const fileListView = createApp(DiffFileList);
  fileListView.mount(fileListElement);
}
