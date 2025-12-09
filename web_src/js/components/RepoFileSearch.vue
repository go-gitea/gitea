<script lang="ts" setup>
import {ref, computed, watch, nextTick, useTemplateRef, onMounted, onUnmounted, type ShallowRef} from 'vue';
import {generateElemId} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';
import {filterRepoFilesWeighted} from '../features/repo-findfile.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import {SvgIcon} from '../svg.ts';
import {throttle} from 'throttle-debounce';

const props = defineProps({
  repoLink: { type: String, required: true },
  currentRefNameSubURL: { type: String, required: true },
  treeListUrl: { type: String, required: true },
  noResultsText: { type: String, required: true },
  placeholder: { type: String, required: true },
});

const refElemInput = useTemplateRef('searchInput') as Readonly<ShallowRef<HTMLInputElement>>;
const refElemPopup = useTemplateRef('searchPopup') as Readonly<ShallowRef<HTMLDivElement>>;

const searchQuery = ref('');
const allFiles = ref<string[]>([]);
const selectedIndex = ref(0);
const isLoadingFileList = ref(false);
const hasLoadedFileList = ref(false);

const showPopup = computed(() => searchQuery.value.length > 0);

const filteredFiles = computed(() => {
  if (!searchQuery.value) return [];
  return filterRepoFilesWeighted(allFiles.value, searchQuery.value);
});

const applySearchQuery = throttle(300, () => {
  searchQuery.value = refElemInput.value.value;
  selectedIndex.value = 0;
});

const handleSearchInput = () => {
  loadFileListForSearch();
  applySearchQuery();
};

const handleKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Escape') {
    e.preventDefault();
    clearSearch();
    return;
  }
  if (!searchQuery.value || filteredFiles.value.length === 0) return;

  const handleSelectedItem = (idx: number) => {
    e.preventDefault();
    selectedIndex.value = idx;
    const el = refElemPopup.value.querySelector(`.file-search-results > :nth-child(${idx+1} of .item)`);
    el?.scrollIntoView({ block: 'nearest', behavior: 'instant' });
  };

  if (e.key === 'ArrowDown') {
    handleSelectedItem(Math.min(selectedIndex.value + 1, filteredFiles.value.length - 1));
  } else if (e.key === 'ArrowUp') {
    handleSelectedItem(Math.max(selectedIndex.value - 1, 0))
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
  refElemInput.value.value = '';
};


const handleClickOutside = (e: MouseEvent) => {
  if (!searchQuery.value) return;

  const target = e.target as HTMLElement;
  const clickInside = refElemInput.value.contains(target) || refElemPopup.value.contains(target);
  if (!clickInside) clearSearch();
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

function handleSearchResultClick(filePath: string) {
  clearSearch();
  window.location.href = `${props.repoLink}/src/${pathEscapeSegments(props.currentRefNameSubURL)}/${pathEscapeSegments(filePath)}`;
}

const updatePosition = () => {
  if (!showPopup.value) return;

  const rectInput = refElemInput.value.getBoundingClientRect();
  const rectPopup = refElemPopup.value.getBoundingClientRect();
  const docElem = document.documentElement;
  const style = refElemPopup.value.style;
  style.top = `${docElem.scrollTop + rectInput.bottom + 4}px`;
  if (rectInput.x + rectPopup.width < docElem.clientWidth) {
    // enough space to align left with the input
    style.left = `${docElem.scrollLeft + rectInput.x}px`;
  } else {
    // no enough space, align right from the viewport right edge minus page margin
    const leftPos = docElem.scrollLeft + docElem.getBoundingClientRect().width - rectPopup.width;
    style.left = `calc(${leftPos}px - var(--page-margin-x))`;
  }
};

onMounted(() => {
  const searchPopupId = generateElemId('file-search-popup-');
  refElemPopup.value.setAttribute('id', searchPopupId);
  refElemInput.value.setAttribute('aria-controls', searchPopupId);
  document.addEventListener('click', handleClickOutside);
  window.addEventListener('resize', updatePosition);
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
  window.removeEventListener('resize', updatePosition);
});

// Position search results below the input
watch([searchQuery, filteredFiles], async () => {
  if (searchQuery.value) {
    await nextTick();
    updatePosition();
  }
});
</script>

<template>
  <div>
    <div class="ui small input">
      <input
        ref="searchInput" :placeholder="placeholder" autocomplete="off"
        role="combobox" aria-autocomplete="list" :aria-expanded="searchQuery ? 'true' : 'false'"
        @input="handleSearchInput" @keydown="handleKeyDown"
      >
    </div>

    <Teleport to="body">
      <div v-show="showPopup" ref="searchPopup" class="file-search-popup">
        <!-- always create the popup by v-show above to avoid null ref, only create the popup content if the popup should be displayed to save memory -->
        <template v-if="showPopup">
          <div v-if="filteredFiles.length" role="listbox" class="file-search-results flex-items-block">
            <div
              v-for="(result, idx) in filteredFiles" :key="result.matchResult.join('')"
              :class="['item', { 'selected': idx === selectedIndex }]"
              role="option" :aria-selected="idx === selectedIndex" @click="handleSearchResultClick(result.matchResult.join(''))"
              @mouseenter="selectedIndex = idx" :title="result.matchResult.join('')"
            >
              <SvgIcon name="octicon-file" class="file-icon"/>
              <span class="full-path">
                <span v-for="(part, index) in result.matchResult" :key="index">{{ part }}</span>
              </span>
            </div>
          </div>
          <div v-else-if="isLoadingFileList">
            <div class="is-loading"/>
          </div>
          <div v-else class="tw-p-4">
            {{ props.noResultsText }}
          </div>
        </template>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.file-search-popup {
  position: absolute;
  background: var(--color-box-body);
  border: 1px solid var(--color-secondary);
  border-radius: var(--border-radius);
  width: max-content;
  max-height: min(calc(100vw - 20px), 300px);
  max-width: min(calc(100vw - 40px), 600px);
  overflow-y: auto;
}

.file-search-popup .is-loading {
  width: 200px;
  height: 200px;
}

.file-search-results .item {
  align-items: flex-start;
  padding: 0.5rem 0.75rem;
  cursor: pointer;
  border-bottom: 1px solid var(--color-secondary);
}

.file-search-results .item:last-child {
  border-bottom: none;
}

.file-search-results .item:hover,
.file-search-results .item.selected {
  background-color: var(--color-hover);
}

.file-search-results .item .file-icon {
  flex-shrink: 0;
  margin-top: 0.125rem;
}

.file-search-results .item .full-path {
  flex: 1;
  overflow-wrap: anywhere;
}

.file-search-results .item .full-path :nth-child(even) {
  color: var(--color-red);
  font-weight: var(--font-weight-semibold);
}
</style>
