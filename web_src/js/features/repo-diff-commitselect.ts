import {createApp} from 'vue';
import DiffCommitSelector from '../components/DiffCommitSelector.vue';

export function initDiffCommitSelect() {
  const el = document.querySelector('#diff-commit-select');
  if (!el) return;

  const commitSelect = createApp(DiffCommitSelector);
  commitSelect.mount(el);
}
