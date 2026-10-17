<script lang="ts" setup>
import SvgIcon from './SvgIcon.vue';
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import DiffFileFilter from './DiffFileFilter.vue';
import {toggleElem} from '../utils/dom.ts';
import {diffTreeStore, filterDiffTree, filteredFileNames, extensionFilterToUrl, type DiffFileTreeLocale} from '../modules/diff-file.ts';
import {onMounted, onUnmounted, computed, watch} from 'vue';
import {localUserSettings} from '../modules/user-settings.ts';

const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

const props = defineProps<{locale: DiffFileTreeLocale}>();

const store = diffTreeStore();

const visibleTreeItems = computed(() => filterDiffTree(store)?.Children ?? []);

// the comments tab lists the threads of the files the filters show
const visibleCommentFiles = computed(() => {
  const names = filteredFileNames(store);
  return (store.commentFiles ?? []).filter((file) => !names || names.has(file.name));
});
const commentThreadCount = computed(() => (store.commentFiles ?? []).reduce((sum, file) => sum + file.threads.length, 0));

watch(() => store.activeExtensions, () => {
  window.history.replaceState(null, '', extensionFilterToUrl(store.activeExtensions, window.location.href));
});

onMounted(() => {
  // Default to true if unset
  store.fileTreeIsVisible = localUserSettings.getBoolean(LOCAL_STORAGE_KEY, true);
  // while the tree is hidden there is no control to clear a filter restored from the URL
  if (!store.fileTreeIsVisible) store.activeExtensions = 'all';
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
}

function toggleVisibility() {
  updateVisibility(!store.fileTreeIsVisible);
}

function updateVisibility(visible: boolean) {
  store.fileTreeIsVisible = visible;
  if (!visible) {
    store.filenameFilterQuery = '';
    store.activeExtensions = 'all';
    store.activeStatuses = 'all';
    store.commentedOnly = false;
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
    <div v-if="store.commentFiles" class="ui small secondary pointing menu tw-m-0" role="tablist">
      <button
        type="button" class="item" role="tab" :class="{active: store.activeTab === 'files'}"
        :aria-selected="store.activeTab === 'files'" @click="store.activeTab = 'files'"
      >
        {{ props.locale.files }}
      </button>
      <button
        type="button" class="item" role="tab" :class="{active: store.activeTab === 'comments'}"
        :aria-selected="store.activeTab === 'comments'" @click="store.activeTab = 'comments'"
      >
        {{ props.locale.comments }}
        <span class="ui small label">{{ commentThreadCount }}</span>
      </button>
    </div>
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
      <DiffFileFilter :locale="props.locale"/>
    </div>
    <div v-if="store.activeTab === 'comments' && store.commentFiles" class="diff-file-tree-items" role="tabpanel">
      <div v-if="!visibleCommentFiles.length" class="tw-p-2 tw-text-text-light-2">{{ props.locale.noComments }}</div>
      <div v-for="file in visibleCommentFiles" :key="file.name" class="tw-flex tw-flex-col tw-gap-px tw-mb-1.5">
        <a class="silenced interact-bg tw-rounded gt-ellipsis tw-py-1 tw-px-1.5 tw-text-12 tw-font-semibold" :href="'#diff-' + store.fullNameMap[file.name].NameHash" :title="file.name">{{ file.name }}</a>
        <a
          v-for="thread in file.threads" :key="thread.id"
          class="silenced interact-bg tw-rounded tw-flex tw-items-center tw-gap-1.5 tw-py-1 tw-pr-1.5 tw-pl-3" :class="{'tw-opacity-60': thread.resolved}" :href="'#issuecomment-' + thread.id"
        >
          <img v-if="thread.avatarUrl" class="ui avatar" :src="thread.avatarUrl" alt="" width="20" height="20">
          <span class="tw-flex-1 tw-min-w-0">
            <span class="tw-flex tw-items-center tw-gap-1">
              <span class="gt-ellipsis tw-font-semibold">{{ thread.posterName }}</span>
              <span v-if="thread.pending" class="ui mini label">{{ props.locale.pending }}</span>
              <span v-if="thread.outdated" class="ui mini label">{{ props.locale.outdated }}</span>
              <span v-if="thread.resolved" class="ui mini label">{{ props.locale.resolved }}</span>
            </span>
            <span class="gt-ellipsis tw-block tw-text-text-light-2">{{ thread.excerpt }}</span>
          </span>
          <span v-if="thread.count > 1" class="tw-flex tw-items-center tw-gap-1 tw-text-text-light-2">
            <SvgIcon name="octicon-comment" :size="12"/>{{ thread.count }}
          </span>
        </a>
      </div>
    </div>
    <div v-else class="diff-file-tree-items">
      <DiffFileTreeItem v-for="item in visibleTreeItems" :key="item.FullName" :item="item"/>
    </div>
  </div>
</template>

<style scoped>
.diff-file-tree-wrapper {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  flex: 1;
  min-height: 0;
}

.diff-file-tree-search-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 1px; /* match .diff-file-box's top border so this row aligns with .diff-file-header */
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
