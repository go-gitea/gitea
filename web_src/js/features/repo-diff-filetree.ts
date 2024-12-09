import {createApp} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {diffTreeStore} from '../modules/stores.ts';
import {setFileFolding} from './file-fold.ts';
import DiffFileList from '../components/DiffFileList.vue';

const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

function hashChangeListener() {
  for (const el of document.querySelectorAll<HTMLAnchorElement>('.file-tree-items .item-file')) {
    el.classList.toggle('selected', el.hash === `${window.location.hash}`);
  }
  expandSelectedFile(window.location.hash);
}

function expandSelectedFile(selectedItem) {
  // expand file if the selected file is folded
  if (selectedItem) {
    const box = document.querySelector(selectedItem);
    const folded = box?.getAttribute('data-folded') === 'true';
    if (folded) setFileFolding(box, box.querySelector('.fold-file'), false);
  }
}

function updateState(visible) {
  const btn = document.querySelector('.diff-toggle-file-tree-button');
  const [toShow, toHide] = btn.querySelectorAll('.icon');
  const tree = document.querySelector('#diff-file-tree');
  const newTooltip = btn.getAttribute(visible ? 'data-hide-text' : 'data-show-text');
  btn.setAttribute('data-tooltip-content', newTooltip);
  toggleElem(tree, visible);
  toggleElem(toShow, !visible);
  toggleElem(toHide, visible);
}

export function initDiffFileTree() {
  const el = document.querySelector('#diff-file-tree');
  if (!el) return;

  const store = diffTreeStore();
  store.fileTreeIsVisible = localStorage.getItem(LOCAL_STORAGE_KEY) !== 'false';
  document.querySelector('.diff-toggle-file-tree-button').addEventListener('click', () => {
    store.fileTreeIsVisible = !store.fileTreeIsVisible;
    localStorage.setItem(LOCAL_STORAGE_KEY, store.fileTreeIsVisible);
    updateState(store.fileTreeIsVisible);
  });

  hashChangeListener();
  window.addEventListener('hashchange', hashChangeListener);

  for (const el of document.querySelectorAll<HTMLInputElement>('.file-tree-items .item-directory')) {
    el.addEventListener('click', () => {
      toggleElem(el.nextElementSibling);
    });
  }
}

export function initDiffFileList() {
  const fileListElement = document.querySelector('#diff-file-list');
  if (!fileListElement) return;

  const fileListView = createApp(DiffFileList);
  fileListView.mount(fileListElement);
}
