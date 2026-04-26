<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import DiffFileExtensionFilter from './DiffFileExtensionFilter.vue';
import {toggleElem} from '../utils/dom.ts';
import {diffTreeStore, isDiffTreeEntryVisible, applyFiltersToFileBoxes} from '../modules/diff-file.ts';
import {setFileFolding} from '../features/file-fold.ts';
import {onMounted, onUnmounted, computed, watch} from 'vue';
import {localUserSettings} from '../modules/user-settings.ts';

const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

const store = diffTreeStore();

const el = document.querySelector<HTMLElement>('#diff-file-tree')!;

function requireAttr(name: string): string {
  const val = el.getAttribute(name);
  if (val === null) throw new Error(`#diff-file-tree is missing required attribute: ${name}`);
  return val;
}

const filterFilesPlaceholder = requireAttr('data-filter-files');
const filterFilesNoResults = requireAttr('data-filter-files-no-results');

// Extension filter locale — only present when the template adds the data attributes (PageIsPullFiles)
const extensionFilterLocale = el.hasAttribute('data-filter-by-file-extension') ? {
  filter_by_file_extension: requireAttr('data-filter-by-file-extension'),
  select_all: requireAttr('data-select-all'),
  deselect_all: requireAttr('data-deselect-all'),
  search: requireAttr('data-search'),
  no_file_extension: requireAttr('data-no-file-extension'),
  no_file_extensions_found: requireAttr('data-no-file-extensions-found'),
} : null;

const visibleTreeItems = computed(() => {
  return (store.diffFileTree.TreeRoot.Children ?? []).filter((item) => isDiffTreeEntryVisible(store, item));
});

const hasSearchQuery = computed(() => Boolean(store.filenameFilterQuery.trim()));

watch(
  () => [store.filenameFilterQuery, store.activeExtensions] as const,
  () => applyFiltersToFileBoxes(store),
);

function clearSearch() {
  store.filenameFilterQuery = '';
}

onMounted(() => {
  store.fileTreeIsVisible = localUserSettings.getBoolean(LOCAL_STORAGE_KEY, true);
  store.noFileExtensionLabel = extensionFilterLocale?.no_file_extension || '';
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
        <input
          type="text"
          v-model="store.filenameFilterQuery"
          class="diff-file-search-input"
          :placeholder="filterFilesPlaceholder"
        >
        <button
          v-if="hasSearchQuery"
          type="button"
          class="diff-file-search-clear"
          @click="clearSearch"
          aria-label="Clear search"
        >
          <SvgIcon name="octicon-x" :size="14"/>
        </button>
      </div>
      <DiffFileExtensionFilter v-if="extensionFilterLocale" :locale="extensionFilterLocale"/>
    </div>
    <div class="diff-file-tree-items">
      <DiffFileTreeItem v-for="item in visibleTreeItems" :key="item.FullName" :item="item" :is-visible="(entry) => isDiffTreeEntryVisible(store, entry)"/>
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
}

.diff-file-tree-search-row {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  position: sticky;
  top: 0;
  background: var(--color-body);
  z-index: 1;
  padding-bottom: 0.25rem;
}

.diff-file-search-wrapper {
  flex: 1;
  min-width: 0;
  position: relative;
  display: flex;
  align-items: center;
}

.diff-file-search-input {
  flex: 1;
  min-width: 0;
  padding: 0.375rem 2.8rem 0.375rem 0.5rem;
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
  right: 0;
  top: 0;
  bottom: 0;
  width: 2.5rem;
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

.diff-file-search-clear :deep(svg) {
  display: block;
}

.diff-file-search-clear:hover {
  color: var(--color-text);
}

.diff-file-tree-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-right: .5rem;
}
</style>
