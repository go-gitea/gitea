<script lang="ts" setup>
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {toggleElem} from '../utils/dom.ts';
import {diffTreeStore} from '../modules/stores.ts';
import {setFileFolding} from '../features/file-fold.ts';
import {computed, onMounted, onUnmounted} from 'vue';
import {pathListToTree, mergeChildIfOnlyOneDir} from '../utils/filetree.ts';

const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

const store = diffTreeStore();

const fileTree = computed(() => {
  const result = pathListToTree(store.files);
  mergeChildIfOnlyOneDir(result); // mutation
  return result;
});

onMounted(() => {
  // Default to true if unset
  store.fileTreeIsVisible = localStorage.getItem(LOCAL_STORAGE_KEY) !== 'false';
  document.querySelector('.diff-toggle-file-tree-button').addEventListener('click', toggleVisibility);

  hashChangeListener();
  window.addEventListener('hashchange', hashChangeListener);
});

onUnmounted(() => {
  document.querySelector('.diff-toggle-file-tree-button').removeEventListener('click', toggleVisibility);
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

function updateVisibility(visible: boolean) {
  store.fileTreeIsVisible = visible;
  localStorage.setItem(LOCAL_STORAGE_KEY, store.fileTreeIsVisible);
  updateState(store.fileTreeIsVisible);
}

function updateState(visible: boolean) {
  const btn = document.querySelector('.diff-toggle-file-tree-button');
  const [toShow, toHide] = btn.querySelectorAll('.icon');
  const tree = document.querySelector('#diff-file-tree');
  const newTooltip = btn.getAttribute(visible ? 'data-hide-text' : 'data-show-text');
  btn.setAttribute('data-tooltip-content', newTooltip);
  toggleElem(tree, visible);
  toggleElem(toShow, !visible);
  toggleElem(toHide, visible);
}
</script>

<template>
  <div v-if="store.fileTreeIsVisible" class="diff-file-tree-items">
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <DiffFileTreeItem v-for="item in fileTree" :key="item.name" :item="item"/>
  </div>
</template>

<style scoped>
.diff-file-tree-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-right: .5rem;
}
</style>
