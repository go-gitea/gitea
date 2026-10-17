import {createApp} from 'vue';
import DiffFileTree from '../components/DiffFileTree.vue';
import type {DiffFileTreeLocale} from '../modules/diff-file.ts';

export function initDiffFileTree() {
  const el = document.querySelector('#diff-file-tree');
  if (!el) return;

  const locale = JSON.parse(el.getAttribute('data-locale')!) as DiffFileTreeLocale;
  createApp(DiffFileTree, {locale}).mount(el);
}
