import {createApp} from 'vue';
import DiffFileTree from '../components/DiffFileTree.vue';

export function initDiffFileTree() {
  const el = document.querySelector('#diff-file-tree');
  if (!el) return;

  const fileTreeView = createApp(DiffFileTree);
  fileTreeView.mount(el);
}
