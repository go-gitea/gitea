<script lang="ts" setup>
import {ref, computed, watch, nextTick, useTemplateRef, onMounted, onUnmounted} from 'vue';
import {GET} from '../modules/fetch.ts';
import {filterRepoFilesWeighted} from '../features/repo-findfile.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import {svg} from '../svg.ts';

const searchInput = useTemplateRef('searchInput');
const searchResults = useTemplateRef('searchResults');
const searchQuery = ref('');
const allFiles = ref<string[]>([]);
const selectedIndex = ref(0);
const isLoadingFileList = ref(false);
const hasLoadedFileList = ref(false);

const props = defineProps({
  repoLink: {type: String, required: true},
  currentRefNameSubURL: {type: String, required: true},
  treeListUrl: {type: String, required: true},
  noResultsText: {type: String, required: true},
  placeholder: {type: String, required: true},
});

const filteredFiles = computed(() => {
  if (!searchQuery.value) return [];
  return filterRepoFilesWeighted(allFiles.value, searchQuery.value);
});

const treeLink = computed(() => `${props.repoLink}/src/${pathEscapeSegments(props.currentRefNameSubURL)}`);

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
  if (searchInput.value) searchInput.value.value = '';
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
  const inputEl = searchInput.value;
  
  if (inputEl && !inputEl.contains(target) && 
      resultsEl && !resultsEl.contains(target)) {
    clearSearch();
  }
};

const loadFileListForSearch = async () => {
  if (hasLoadedFileList.value || isLoadingFileList.value) return;
  
  isLoadingFileList.value = true;
  try {
    const response = await GET(props.treeListUrl);
    allFiles.value = await response.json();
    hasLoadedFileList.value = true;
  } finally {
    isLoadingFileList.value = false;
  }
};

const handleSearchFocus = () => {
  loadFileListForSearch();
};

function handleSearchResultClick(filePath: string) {
  clearSearch();
  window.location.href = `${treeLink.value}/${pathEscapeSegments(filePath)}`;
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
});

// Position search results below the input
watch(searchQuery, async () => {
  if (searchQuery.value && searchInput.value) {
    await nextTick();
    const resultsEl = searchResults.value;
    if (resultsEl) {
      const rect = searchInput.value.getBoundingClientRect();
      resultsEl.style.top = `${rect.bottom + 4}px`;
      resultsEl.style.left = `${rect.left}px`;
    }
  }
});
</script>

<template>
  <div class="repo-file-search">
    <div class="ui small input tw-w-full tw-px-2 tw-pb-2">
      <input 
        ref="searchInput"
        type="text" 
        :placeholder="placeholder" 
        autocomplete="off"
        @input="handleSearchInput"
        @keydown="handleKeyDown"
        @focus="handleSearchFocus"
      >
    </div>
    
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
          <span>{{ props.noResultsText }}</span>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.repo-file-search {
  position: relative;
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
