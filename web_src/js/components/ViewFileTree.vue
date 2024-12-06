<script lang="ts" setup>
import FileTree from './FileTree.vue';
import {GET} from '../modules/fetch.ts';
import {toggleElem} from '../utils/dom.ts';
import {ref, onMounted, onUnmounted} from 'vue';

const files = ref(null);
const visible = ref(false);
const isLoading = ref(false);

onMounted(async () => {
  // Default to false
  updateVisibility(false);
  document.querySelector('.view-toggle-file-tree-button').addEventListener('click', toggleVisibility);

  isLoading.value = true;
  const children = await loadChildren();
  files.value = children;
  isLoading.value = false;
});

onUnmounted(() => {
  document.querySelector('.view-toggle-file-tree-button').removeEventListener('click', toggleVisibility);
});

function toggleVisibility() {
  updateVisibility(!visible.value);
}

function updateVisibility(_visible) {
  visible.value = _visible;
  updateState(visible.value);
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
  const el = document.querySelector('#view-file-tree');
  const apiBaseUrl = el.getAttribute('data-api-base-url');
  const response = await GET(`${apiBaseUrl}/contents/${item ? item.file.path : ''}`);
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
    v-if="visible"
    id="view-file-tree"
    :is-loading="isLoading"
    :files="files"
    :collapsed="true"
    :file-url-getter="item => item.file.html_url"
    :load-children="loadChildren"
  />
</template>
