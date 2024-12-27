<script lang="ts" setup>
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {loadMoreFiles} from '../features/repo-diff.ts';
import {toggleElem} from '../utils/dom.ts';
import {diffTreeStore} from '../modules/stores.ts';
import {setFileFolding} from '../features/file-fold.ts';
import {computed, onMounted, onUnmounted} from 'vue';

const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

const store = diffTreeStore();

const fileTree = computed(() => {
  const result = [];
  for (const file of store.files) {
    // Split file into directories
    const splits = file.Name.split('/');
    let index = 0;
    let parent = null;
    let isFile = false;
    for (const split of splits) {
      index += 1;
      // reached the end
      if (index === splits.length) {
        isFile = true;
      }
      let newParent = {
        name: split,
        children: [],
        isFile,
      } as {
        name: string,
        children: any[],
        isFile: boolean,
        file?: any,
      };

      if (isFile === true) {
        newParent.file = file;
      }

      if (parent) {
        // check if the folder already exists
        const existingFolder = parent.children.find(
          (x) => x.name === split,
        );
        if (existingFolder) {
          newParent = existingFolder;
        } else {
          parent.children.push(newParent);
        }
      } else {
        const existingFolder = result.find((x) => x.name === split);
        if (existingFolder) {
          newParent = existingFolder;
        } else {
          result.push(newParent);
        }
      }
      parent = newParent;
    }
  }
  const mergeChildIfOnlyOneDir = (entries) => {
    for (const entry of entries) {
      if (entry.children) {
        mergeChildIfOnlyOneDir(entry.children);
      }
      if (entry.children.length === 1 && entry.children[0].isFile === false) {
        // Merge it to the parent
        entry.name = `${entry.name}/${entry.children[0].name}`;
        entry.children = entry.children[0].children;
      }
    }
  };
  // Merge folders with just a folder as children in order to
  // reduce the depth of our tree.
  mergeChildIfOnlyOneDir(result);
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

function updateVisibility(visible) {
  store.fileTreeIsVisible = visible;
  localStorage.setItem(LOCAL_STORAGE_KEY, store.fileTreeIsVisible);
  updateState(store.fileTreeIsVisible);
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

function loadMoreData() {
  loadMoreFiles(store.linkLoadMore);
}
</script>

<template>
  <div v-if="store.fileTreeIsVisible" class="diff-file-tree-items">
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <DiffFileTreeItem v-for="item in fileTree" :key="item.name" :item="item"/>
    <div v-if="store.isIncomplete" class="tw-pt-1">
      <a :class="['ui', 'basic', 'tiny', 'button', store.isLoadingNewData ? 'disabled' : '']" @click.stop="loadMoreData">{{ store.showMoreMessage }}</a>
    </div>
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
