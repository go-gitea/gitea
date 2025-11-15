<script lang="ts" setup>
import ViewFileTreeItem from './ViewFileTreeItem.vue';
import {onMounted, onUnmounted, useTemplateRef, ref, computed, watch, nextTick} from 'vue';
import {createViewFileTreeStore} from './ViewFileTreeStore.ts';
import {GET} from '../modules/fetch.ts';
import {filterRepoFilesWeighted} from '../features/repo-findfile.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import {svg} from '../svg.ts';

const elRoot = useTemplateRef('elRoot');
const searchResults = useTemplateRef('searchResults');
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
  if (e.key === 'Escape' && searchQuery.value) {
    e.preventDefault();
    clearSearch();
    return;
  }

  if (!searchQuery.value || filteredFiles.value.length === 0) return;

  if (e.key === 'ArrowDown') {
    e.preventDefault();
    selectedIndex.value = Math.min(selectedIndex.value + 1, filteredFiles.value.length - 1);
    scrollSelectedIntoView();
  } else if (e.key === 'ArrowUp') {
    e.preventDefault();
    selectedIndex.value = Math.max(selectedIndex.value - 1, 0);
    scrollSelectedIntoView();
  } else if (e.key === 'Enter') {
    e.preventDefault();
    const selectedFile = filteredFiles.value[selectedIndex.value];
    if (selectedFile) {
      handleSearchResultClick(selectedFile.matchResult.join(''));
    }
  }
};

const clearSearch = () => {
  searchQuery.value = '';
  if (searchInputElement) searchInputElement.value = '';
};

const scrollSelectedIntoView = () => {
  nextTick(() => {
    const resultsEl = searchResults.value;
    if (!resultsEl) return;
    
    const selectedEl = resultsEl.querySelector('.file-tree-search-result-item.selected');
    if (selectedEl) {
      selectedEl.scrollIntoView({block: 'nearest', behavior: 'smooth'});
    }
  });
};

const handleClickOutside = (e: MouseEvent) => {
  if (!searchQuery.value) return;
  
  const target = e.target as HTMLElement;
  const resultsEl = searchResults.value;
  
  // Check if click is outside search input and results
  if (searchInputElement && !searchInputElement.contains(target) && 
      resultsEl && !resultsEl.contains(target)) {
    clearSearch();
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
  
  // Add click outside listener
  document.addEventListener('click', handleClickOutside);
  
  window.addEventListener('popstate', (e) => {
    store.selectedItem = e.state?.treePath || '';
    if (e.state?.url) store.loadViewContent(e.state.url);
  });
});

// Position search results below the input
watch(searchQuery, async () => {
  if (searchQuery.value && searchInputElement) {
    await nextTick();
    const resultsEl = searchResults.value;
    if (resultsEl) {
      const rect = searchInputElement.getBoundingClientRect();
      resultsEl.style.top = `${rect.bottom + 4}px`;
      resultsEl.style.left = `${rect.left}px`;
    }
  }
});

onUnmounted(() => {
  if (searchInputElement) {
    searchInputElement.removeEventListener('input', handleSearchInput);
    searchInputElement.removeEventListener('keydown', handleKeyDown);
  }
  document.removeEventListener('click', handleClickOutside);
});

function handleSearchResultClick(filePath: string) {
  clearSearch();
  window.location.href = `${treeLink.value}/${pathEscapeSegments(filePath)}`;
}
</script>

<template>
  <div ref="elRoot" class="file-tree-root">
    <Teleport to="body">
      <div v-if="searchQuery && filteredFiles.length > 0" ref="searchResults" class="file-tree-search-results">
        <div 
          v-for="(result, idx) in filteredFiles" 
          :key="result.matchResult.join('')"
          :class="['file-tree-search-result-item', {'selected': idx === selectedIndex}]"
          @click="handleSearchResultClick(result.matchResult.join(''))"
          @mouseenter="selectedIndex = idx"
          :title="result.matchResult.join('')"
        >
          <!-- eslint-disable-next-line vue/no-v-html -->
          <span v-html="svg('octicon-file', 16)"/>
          <span class="file-tree-search-result-path">
            <span 
              v-for="(part, index) in result.matchResult" 
              :key="index"
              :class="{'search-match': index % 2 === 1}"
            >{{ part }}</span>
          </span>
        </div>
      </div>
      <div v-if="searchQuery && filteredFiles.length === 0" ref="searchResults" class="file-tree-search-results file-tree-search-no-results">
        <div class="file-tree-no-results-content">
          <!-- eslint-disable-next-line vue/no-v-html -->
          <span v-html="svg('octicon-search', 24)"/>
          <span>No matching files found</span>
        </div>
      </div>
    </Teleport>
    <div class="view-file-tree-items">
      <ViewFileTreeItem v-for="item in store.rootFiles" :key="item.name" :item="item" :store="store"/>
    </div>
  </div>
</template>

<style scoped>
.file-tree-root {
  position: relative;
}

.view-file-tree-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-right: .5rem;
}

.file-tree-search-results {
  position: fixed;
  display: flex;
  flex-direction: column;
  max-height: 400px;
  overflow-y: auto;
  background: var(--color-box-body);
  border: 1px solid var(--color-secondary);
  border-radius: 6px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  min-width: 300px;
  width: max-content;
  max-width: 600px;
  z-index: 99999;
}

.file-tree-search-result-item {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  cursor: pointer;
  transition: background-color 0.1s;
  border-bottom: 1px solid var(--color-secondary);
}

.file-tree-search-result-item > span:first-child {
  flex-shrink: 0;
  margin-top: 0.125rem;
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
  font-size: 14px;
  word-break: break-all;
  overflow-wrap: break-word;
}

.search-match {
  color: var(--color-red);
  font-weight: var(--font-weight-semibold);
}

.file-tree-search-no-results {
  padding: 0;
}

.file-tree-no-results-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 1.5rem;
  color: var(--color-text-light-2);
  font-size: 14px;
}

.file-tree-no-results-content > span:first-child {
  opacity: 0.5;
}
</style>
