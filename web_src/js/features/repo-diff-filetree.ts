import {createApp} from 'vue';
import DiffFileTree from '../components/DiffFileTree.vue';
import type {DiffFileTreeLocale} from '../modules/diff-file.ts';

export function initDiffFileTree() {
  const el = document.querySelector('#diff-file-tree');
  if (!el) return;

  const locale: DiffFileTreeLocale = {
    filterFiles: el.getAttribute('data-text-filter-files')!,
    filterFilesClear: el.getAttribute('data-text-filter-files-clear')!,
    noFilesMatched: el.getAttribute('data-text-no-files-matched')!,
    filterByFileExtension: el.getAttribute('data-text-filter-by-file-extension')!,
    fileExtensions: el.getAttribute('data-text-file-extensions')!,
    noFileExtension: el.getAttribute('data-text-no-file-extension')!,
    allFileExtensions: el.getAttribute('data-text-all-file-extensions')!,
  };

  const fileTreeView = createApp(DiffFileTree, {locale});
  fileTreeView.mount(el);
}
