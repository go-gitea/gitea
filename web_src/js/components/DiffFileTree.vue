<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import DiffFileExtensionFilter from './DiffFileExtensionFilter.vue';
import {toggleElem} from '../utils/dom.ts';
import {diffTreeStore, filterDiffTree, applyFiltersToFileBoxes} from '../modules/diff-file.ts';
import {setFileFolding} from '../features/file-fold.ts';
import {onMounted, onUnmounted, computed, watch} from 'vue';
import {localUserSettings} from '../modules/user-settings.ts';

const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

const store = diffTreeStore();

const el = document.querySelector<HTMLElement>('#diff-file-tree')!;

const filterFilesPlaceholder = el.getAttribute('data-filter-files')!;
const filterFilesNoResults = el.getAttribute('data-filter-files-no-results')!;
const filterFilesClearLabel = el.getAttribute('data-filter-files-clear')!;

const visibleTreeItems = computed(() => filterDiffTree(store)?.Children ?? []);

watch(
  () => [store.filenameFilterQuery, store.activeExtensions] as const,
  () => applyFiltersToFileBoxes(store),
);

function clearSearch() {
  store.filenameFilterQuery = '';
}

let fileBoxesObserver: MutationObserver;

onMounted(() => {
  store.fileTreeIsVisible = localUserSettings.getBoolean(LOCAL_STORAGE_KEY, true);
  document.querySelector('.diff-toggle-file-tree-button')!.addEventListener('click', toggleVisibility);

  fileBoxesObserver = new MutationObserver(() => applyFiltersToFileBoxes(store));
  fileBoxesObserver.observe(document.querySelector('#diff-file-boxes')!, {childList: true});

  hashChangeListener();
  window.addEventListener('hashchange', hashChangeListener);
});

onUnmounted(() => {
  document.querySelector('.diff-toggle-file-tree-button')!.removeEventListener('click', toggleVisibility);
  window.removeEventListener('hashchange', hashChangeListener);
  fileBoxesObserver.disconnect();
});

function hashChangeListener() {
  store.selectedItem = window.location.hash;
  expandSelectedFile();
}

function expandSelectedFile() {
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
          :placeholder="filterFilesPlaceholder"
          :aria-label="filterFilesPlaceholder"
        >
        <button
          v-if="store.filenameFilterQuery"
          type="button"
          class="diff-file-search-clear"
          @click="clearSearch"
          :aria-label="filterFilesClearLabel"
        >
          <SvgIcon name="octicon-x" :size="14"/>
        </button>
      </div>
      <DiffFileExtensionFilter/>
    </div>
    <div class="diff-file-tree-items">
      <DiffFileTreeItem v-for="item in visibleTreeItems" :key="item.FullName" :item="item"/>
      <div v-if="visibleTreeItems.length === 0" class="tw-py-4 tw-text-center tw-text-text-light-2">
        {{ filterFilesNoResults }}
      </div>
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
  gap: 0.375rem;
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
  border-radius: var(--border-radius);
  background: var(--color-input-background);
  color: var(--color-text);
  font-size: 1em;
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
  cursor: pointer;
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
