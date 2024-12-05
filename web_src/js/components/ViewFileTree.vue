<script lang="ts" setup>
import FileTree from './FileTree.vue';
import {GET} from '../modules/fetch.ts';
import {toggleElem} from '../utils/dom.ts';
import {viewTreeStore} from '../modules/stores.ts';
import {setFileFolding} from '../features/file-fold.ts';
import {ref, onMounted, onUnmounted} from 'vue';

const LOCAL_STORAGE_KEY = 'view_file_tree_visible';

const store = viewTreeStore();

const isLoading = ref(false);

onMounted(async () => {
  // Default to true if unset
  updateVisibility(localStorage.getItem(LOCAL_STORAGE_KEY) !== 'false');
  document.querySelector('.view-toggle-file-tree-button').addEventListener('click', toggleVisibility);

  hashChangeListener();
  window.addEventListener('hashchange', hashChangeListener);

  isLoading.value = true;
  const files = await loadChildren();
  store.files = files;
  isLoading.value = false;

  window.localStorage.setItem(`${LOCAL_STORAGE_KEY}-/`, files);
});

onUnmounted(() => {
  document.querySelector('.view-toggle-file-tree-button').removeEventListener('click', toggleVisibility);
  window.removeEventListener('hashchange', hashChangeListener);
});

function hashChangeListener() {
  store.selectedItem = window.location.hash;
  expandSelectedFile();
}

function expandSelectedFile() {
  // expand file if the selected file is folded
  if (store.selectedItem) {
    const box = document.querySelector(store.selectedItem);
    const folded = box?.getAttribute('data-folded') === 'true';
    if (folded) setFileFolding(box, box.querySelector('.fold-file'), false);
  }
}

function toggleVisibility() {
  updateVisibility(!store.fileTreeIsVisible);
}

function updateVisibility(visible) {
  store.fileTreeIsVisible = visible;
  localStorage.setItem(LOCAL_STORAGE_KEY, store.fileTreeIsVisible);
  updateState(store.fileTreeIsVisible);
}

function updateState(visible) {
  const btn = document.querySelector('.view-toggle-file-tree-button');
  const [toShow, toHide] = btn.querySelectorAll('.icon');
  const tree = document.querySelector('#view-file-tree');
  const newTooltip = btn.getAttribute(visible ? 'data-hide-text' : 'data-show-text');
  btn.setAttribute('data-tooltip-content', newTooltip);
  toggleElem(tree, visible);
  toggleElem(toShow, !visible);
  toggleElem(toHide, visible);
}

async function loadChildren(item?) {
  const response = await GET(`/api/v1/repos/${window.config.pageData.viewFileInfo.apiBaseUrl}/contents/${item ? item.file.path : ''}`);
  const json = await response.json();
  return json.map((i) => ({
    file: i,
    name: i.name,
    isFile: i.type === 'file',
  }));
}
</script>

<template>
  <FileTree
    id="view-file-tree"
    :is-loading="isLoading"
    :files="store.files"
    :collapsed="true"
    :visible="store.fileTreeIsVisible"
    :selected="store.selectedItem"
    :file-url-getter="item => item.file.html_url"
    :load-children="loadChildren"
  />
</template>
