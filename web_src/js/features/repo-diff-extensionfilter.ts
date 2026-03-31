import {createApp} from 'vue';
import DiffFileExtensionFilter from '../components/DiffFileExtensionFilter.vue';

export function initDiffFileExtensionFilter() {
  const el = document.querySelector('#diff-extension-filter');
  if (!el) return;

  const extensionFilter = createApp(DiffFileExtensionFilter);
  extensionFilter.mount(el);
}
