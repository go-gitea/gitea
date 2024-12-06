import {createApp} from 'vue';
import ViewFileTree from '../components/ViewFileTree.vue';

export function initViewFileTree() {
  const el = document.querySelector('#view-file-tree');
  if (!el) return;

  const fileTreeView = createApp(ViewFileTree);
  fileTreeView.mount(el);
}
