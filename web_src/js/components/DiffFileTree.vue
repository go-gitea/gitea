<script lang="ts" setup>
import SvgIcon from './SvgIcon.vue';
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import DiffFileExtensionFilter from './DiffFileExtensionFilter.vue';
import {onInputDebounce, toggleElem} from '../utils/dom.ts';
import {diffTreeStore, filterDiffTree, applyFiltersToFileBoxes, extensionFilterToUrl, type DiffFileTreeLocale} from '../modules/diff-file.ts';
import {setFileFolding} from '../features/file-fold.ts';
import {onMounted, onUnmounted, computed, watch} from 'vue';
import {localUserSettings} from '../modules/user-settings.ts';

const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

const props = defineProps<{locale: DiffFileTreeLocale}>();

const store = diffTreeStore();

const visibleTreeItems = computed(() => filterDiffTree(store)?.Children ?? []);

watch(() => store.filenameFilterQuery, onInputDebounce(() => applyFiltersToFileBoxes(store)));
watch(() => store.activeExtensions, () => {
  applyFiltersToFileBoxes(store);
  window.history.replaceState(null, '', extensionFilterToUrl(store.activeExtensions, window.location.href));
});

onMounted(() => {
  // Default to true if unset
  store.fileTreeIsVisible = localUserSettings.getBoolean(LOCAL_STORAGE_KEY, true);
  // while the tree is hidden there is no control to clear a filter restored from the URL
  if (store.fileTreeIsVisible) applyFiltersToFileBoxes(store); else store.activeExtensions = 'all';
  document.querySelector('.diff-toggle-file-tree-button')!.addEventListener('click', toggleVisibility);
  hashChangeListener();
  window.addEventListener('hashchange', hashChangeListener);
});

onUnmounted(() => {
  document.querySelector('.diff-toggle-file-tree-button')!.removeEventListener('click', toggleVisibility);
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
    if (folded) setFileFolding(box, box.querySelector('.fold-file')!, false);
  }
}

function toggleVisibility() {
  updateVisibility(!store.fileTreeIsVisible);
}

function updateVisibility(visible: boolean) {
  store.fileTreeIsVisible = visible;
  if (!visible) {
    store.filenameFilterQuery = '';
    store.activeExtensions = 'all';
    applyFiltersToFileBoxes(store);
  }
  localUserSettings.setBoolean(LOCAL_STORAGE_KEY, store.fileTreeIsVisible);
  updateState(store.fileTreeIsVisible);
}

function updateState(visible: boolean) {
  const btn = document.querySelector('.diff-toggle-file-tree-button')!;
  const [toShow, toHide] = btn.querySelectorAll('.icon');
  const tree = document.querySelector('#diff-file-tree')!;
  const newTooltip = btn.getAttribute(visible ? 'data-hide-text' : 'data-show-text')!;
  btn.setAttribute('data-tooltip-content', newTooltip);
  toggleElem(tree, visible);
  toggleElem(toShow, !visible);
  toggleElem(toHide, visible);
}
</script>

<template>
  <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
  <div v-if="store.fileTreeIsVisible" class="diff-file-tree-wrapper">
    <div class="diff-file-tree-search-row">
      <div class="diff-file-search-wrapper">
        <SvgIcon name="octicon-search" :size="14" class="diff-file-search-icon"/>
        <input
          type="text"
          v-model="store.filenameFilterQuery"
          class="diff-file-search-input"
          :placeholder="props.locale.filterFiles"
          :aria-label="props.locale.filterFiles"
        >
        <button
          v-if="store.filenameFilterQuery"
          type="button"
          class="diff-file-search-clear"
          @click="store.filenameFilterQuery = ''"
          :aria-label="props.locale.filterFilesClear"
        >
          <SvgIcon name="octicon-x" :size="14"/>
        </button>
      </div>
      <DiffFileExtensionFilter :locale="props.locale"/>
    </div>
    <div class="diff-file-tree-items">
      <DiffFileTreeItem v-for="item in visibleTreeItems" :key="item.FullName" :item="item"/>
    </div>
  </div>
</template>

<style scoped>
.diff-file-tree-wrapper {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-right: .5rem;
  flex: 1;
  min-height: 0;
}

.diff-file-tree-search-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 1px; /* match .diff-file-box's top border so this row aligns with .diff-file-header */
  padding-bottom: 0.25rem;
}

.diff-file-search-wrapper {
  flex: 1;
  min-width: 0;
  position: relative;
  display: flex;
  align-items: center;
}

.diff-file-search-icon {
  position: absolute;
  left: 8px;
  color: var(--color-text-light-2);
  pointer-events: none;
}

.diff-file-search-input {
  flex: 1;
  min-width: 0;
  height: 32px;
  padding: 0 28px;
  border: 1px solid var(--color-secondary);
  border-radius: var(--border-radius-medium);
  background: var(--color-input-background);
  color: var(--color-text);
}

.diff-file-search-input:focus {
  outline: none;
  border-color: var(--color-primary);
}

.diff-file-search-clear {
  position: absolute;
  right: 4px;
  top: 0;
  bottom: 0;
  width: 20px;
  background: none;
  border: none;
  color: var(--color-text-light);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: auto 0;
  padding: 0;
}

.diff-file-search-clear:hover {
  color: var(--color-text);
}

.diff-file-tree-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}
</style>
