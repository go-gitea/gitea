<script lang="ts" setup>
import ViewFileTreeItem from './ViewFileTreeItem.vue';
import {onMounted, onUnmounted, useTemplateRef, ref, computed} from 'vue';
import {createViewFileTreeStore} from './ViewFileTreeStore.ts';
import {GET} from '../modules/fetch.ts';
import {filterRepoFilesWeighted} from '../features/repo-findfile.ts';
import {pathEscapeSegments} from '../utils/url.ts';

const elRoot = useTemplateRef('elRoot');
const searchQuery = ref('');
const allFiles = ref<string[]>([]);
const selectedIndex = ref(0);

const props = defineProps({
  repoLink: {type: String, required: true},
  treePath: {type: String, required: true},
  currentRefNameSubURL: {type: String, required: true},
});

const store = createViewFileTreeStore(props);

const filteredFiles = computed(() => {
  if (!searchQuery.value) return [];
  return filterRepoFilesWeighted(allFiles.value, searchQuery.value);
});

const treeLink = computed(() => `${props.repoLink}/src/${props.currentRefNameSubURL}`);

let searchInputElement: HTMLInputElement | null = null;

const handleSearchInput = (e: Event) => {
  searchQuery.value = (e.target as HTMLInputElement).value;
  selectedIndex.value = 0;
};

const handleKeyDown = (e: KeyboardEvent) => {
  if (!searchQuery.value || filteredFiles.value.length === 0) return;

  if (e.key === 'ArrowDown') {
    e.preventDefault();
    selectedIndex.value = Math.min(selectedIndex.value + 1, filteredFiles.value.length - 1);
  } else if (e.key === 'ArrowUp') {
    e.preventDefault();
    selectedIndex.value = Math.max(selectedIndex.value - 1, 0);
  } else if (e.key === 'Enter') {
    e.preventDefault();
    const selectedFile = filteredFiles.value[selectedIndex.value];
    if (selectedFile) {
      handleSearchResultClick(selectedFile.matchResult.join(''));
    }
  } else if (e.key === 'Escape') {
    searchQuery.value = '';
    if (searchInputElement) searchInputElement.value = '';
  }
};

onMounted(async () => {
  store.rootFiles = await store.loadChildren('', props.treePath);
  elRoot.value.closest('.is-loading')?.classList?.remove('is-loading');
  
  // Load all files for search
  const treeListUrl = elRoot.value.closest('#view-file-tree')?.getAttribute('data-tree-list-url');
  if (treeListUrl) {
    const response = await GET(treeListUrl);
    allFiles.value = await response.json();
  }
  
  // Setup search input listener
  searchInputElement = document.querySelector('#file-tree-search');
  if (searchInputElement) {
    searchInputElement.addEventListener('input', handleSearchInput);
    searchInputElement.addEventListener('keydown', handleKeyDown);
  }
  
  window.addEventListener('popstate', (e) => {
    store.selectedItem = e.state?.treePath || '';
    if (e.state?.url) store.loadViewContent(e.state.url);
  });
});

onUnmounted(() => {
  if (searchInputElement) {
    searchInputElement.removeEventListener('input', handleSearchInput);
    searchInputElement.removeEventListener('keydown', handleKeyDown);
  }
});

function handleSearchResultClick(filePath: string) {
  searchQuery.value = '';
  if (searchInputElement) searchInputElement.value = '';
  window.location.href = `${treeLink.value}/${pathEscapeSegments(filePath)}`;
}
</script>

<template>
  <div ref="elRoot">
    <div v-if="searchQuery && filteredFiles.length > 0" class="file-tree-search-results">
      <div 
        v-for="(result, idx) in filteredFiles" 
        :key="result.matchResult.join('')"
        :class="['file-tree-search-result-item', {'selected': idx === selectedIndex}]"
        @click="handleSearchResultClick(result.matchResult.join(''))"
        @mouseenter="selectedIndex = idx"
      >
        <svg class="svg octicon-file" width="16" height="16" aria-hidden="true"><use href="#octicon-file"/></svg>
        <span class="file-tree-search-result-path">
          <span 
            v-for="(part, index) in result.matchResult" 
            :key="index"
            :class="{'search-match': index % 2 === 1}"
          >{{ part }}</span>
        </span>
      </div>
    </div>
    <div v-else-if="searchQuery && filteredFiles.length === 0" class="file-tree-search-no-results">
      No matching file found
    </div>
    <div v-else class="view-file-tree-items">
      <ViewFileTreeItem v-for="item in store.rootFiles" :key="item.name" :item="item" :store="store"/>
    </div>
  </div>
</template>

<style scoped>
.view-file-tree-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-right: .5rem;
}

.file-tree-search-results {
  display: flex;
  flex-direction: column;
  margin: 0 0.5rem 0.5rem;
  max-height: 400px;
  overflow-y: auto;
  background: var(--color-box-body);
  border: 1px solid var(--color-secondary);
  border-radius: 6px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

.file-tree-search-result-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  cursor: pointer;
  transition: background-color 0.1s;
  border-bottom: 1px solid var(--color-secondary);
}

.file-tree-search-result-item:last-child {
  border-bottom: none;
}

.file-tree-search-result-item:hover,
.file-tree-search-result-item.selected {
  background-color: var(--color-hover);
}

.file-tree-search-result-path {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

.search-match {
  color: var(--color-red);
  font-weight: var(--font-weight-semibold);
}

.file-tree-search-no-results {
  padding: 1rem;
  text-align: center;
  color: var(--color-text-light-2);
  font-size: 14px;
}
</style>
