<script lang="ts" setup>
import {computed, nextTick, onMounted, onUnmounted, ref} from 'vue';
import {throttle} from 'throttle-debounce';
import {SvgIcon} from '../svg.ts';
import {generateElemId} from '../utils/dom.ts';
import {extname} from '../utils.ts';

type Extension = {
  ext: string,
  checked: boolean,
  count: number,
}

const el = document.querySelector<HTMLElement>('#diff-extension-filter')!;
const menuVisible = ref(false);
const extensions = ref<Array<Extension>>([]);
const appliedExtensions = ref<Array<string> | null>(null);
const searchQuery = ref('');
const uniqueIdMenu = generateElemId('diff-extension-filter-menu-');
let mutationObserver: MutationObserver | null = null;
const locale = {
  filter_by_file_extension: el.getAttribute('data-filter-by-file-extension'),
  select_all: el.getAttribute('data-select-all'),
  deselect_all: el.getAttribute('data-deselect-all'),
  search: el.getAttribute('data-search'),
  no_file_extension: el.getAttribute('data-no-file-extension'),
  no_file_extensions_found: el.getAttribute('data-no-file-extensions-found'),
} as Record<string, string>;

const isFiltering = computed(() => appliedExtensions.value !== null);

// Subset of extensions shown in the dropdown while the user types in the search box.
// Does not affect which extensions are checked or which files are hidden — only narrows
// the visible list to help the user find a specific extension quickly.
// e.g. searchQuery=".ts" → [.ts, .tsx]; searchQuery="" → all extensions
const filteredExtensions = computed(() => {
  if (!searchQuery.value.trim()) {
    return extensions.value;
  }
  const query = searchQuery.value.toLowerCase();
  return extensions.value.filter((ext) => ext.ext.toLowerCase().includes(query));
});

function fileBoxes(): NodeListOf<HTMLElement> {
  return document.querySelectorAll<HTMLElement>('#diff-file-boxes .diff-file-box[data-new-filename]');
}

function fileExt(box: HTMLElement): string {
  return extname(box.getAttribute('data-new-filename') || '') || locale.no_file_extension;
}

function scanExtensions() {
  const extensionMap = new Map<string, {total: number, visible: number}>();
  for (const box of fileBoxes()) {
    const ext = fileExt(box);
    const stats = extensionMap.get(ext) ?? {total: 0, visible: 0};
    stats.total += 1;
    if (!box.classList.contains('tw-hidden')) stats.visible += 1;
    extensionMap.set(ext, stats);
  }

  extensions.value = Array.from(extensionMap.entries())
    .map(([ext, stats]) => ({
      ext,
      checked: appliedExtensions.value ? appliedExtensions.value.includes(ext) : stats.visible > 0,
      count: stats.total,
    }))
    .sort((a, b) => b.count - a.count);
}

function applyFilterToFileBoxes(checkedExtensions: Set<string>) {
  let hiddenCount = 0;
  for (const box of fileBoxes()) {
    if (checkedExtensions.has(fileExt(box))) {
      box.classList.remove('tw-hidden');
    } else {
      box.classList.add('tw-hidden');
      hiddenCount += 1;
    }
  }
  appliedExtensions.value = hiddenCount > 0 ? Array.from(checkedExtensions) : null;
}

function focusElem(elem: HTMLElement | null, prevElem: HTMLElement | null) {
  if (!elem) return;
  elem.tabIndex = 0;
  if (prevElem) prevElem.tabIndex = -1;
  const focusTarget = elem.querySelector<HTMLElement>('input, button') ?? elem;
  focusTarget.focus();
}

function toggleMenu() {
  menuVisible.value = !menuVisible.value;
  if (menuVisible.value) {
    searchQuery.value = '';
    scanExtensions();
    document.addEventListener('click', onOutsideClick, true);
    nextTick(() => el.querySelector<HTMLInputElement>('.diff-ext-search-input')!.focus());
  } else {
    document.removeEventListener('click', onOutsideClick, true);
  }
}

function selectAll() {
  for (const ext of extensions.value) {
    ext.checked = true;
  }
  applyFilter();
}

function deselectAll() {
  for (const ext of extensions.value) {
    ext.checked = false;
  }
  applyFilter();
}

function applyFilter() {
  const checkedExtensions = new Set(extensions.value.filter((e) => e.checked).map((e) => e.ext));
  applyFilterToFileBoxes(checkedExtensions);
}

function onOutsideClick(event: MouseEvent) {
  if (el.contains(event.target as Node)) return;
  if (menuVisible.value) {
    toggleMenu();
  }
}

function onKeyDown(event: KeyboardEvent) {
  if (!menuVisible.value) return;
  const currentFocused = document.activeElement as HTMLElement;
  if (!el.contains(currentFocused)) return;

  const menu = el.querySelector('.menu') as HTMLElement;
  const focusableItems = Array.from(menu.querySelectorAll('[role="menuitem"]')) as HTMLElement[];

  if (!focusableItems.length) return;

  const currentIndex = focusableItems.indexOf(currentFocused.closest('[role="menuitem"]') as HTMLElement);

  switch (event.key) {
    case 'ArrowDown': {
      event.preventDefault();
      const nextIndex = currentIndex === -1 ? 0 : Math.min(currentIndex + 1, focusableItems.length - 1);
      focusElem(focusableItems[nextIndex], currentIndex >= 0 ? focusableItems[currentIndex] : null);
      break;
    }
    case 'ArrowUp': {
      event.preventDefault();
      const prevIndex = currentIndex === -1 ? focusableItems.length - 1 : Math.max(currentIndex - 1, 0);
      focusElem(focusableItems[prevIndex], currentIndex >= 0 ? focusableItems[currentIndex] : null);
      break;
    }
    case ' ':
    case 'Enter': {
      event.preventDefault();
      const currentElement = document.activeElement as HTMLElement;

      // currentElement may be the checkbox input itself, or a menuitem wrapping it
      const checkbox = (currentElement?.matches('input[type="checkbox"]')
        ? currentElement
        : currentElement?.querySelector('input[type="checkbox"]')) as HTMLInputElement | null;
      if (checkbox) {
        checkbox.checked = !checkbox.checked;
        checkbox.dispatchEvent(new Event('change', {bubbles: true}));
        break;
      }

      if (currentElement?.tagName === 'BUTTON') {
        currentElement.click();
      }
      break;
    }
    case 'Escape':
      event.preventDefault();
      if (currentIndex >= 0) {
        focusableItems[currentIndex].tabIndex = -1;
      }
      toggleMenu();
      break;
  }
}

// Re-apply the filter when files are dynamically added (e.g. "load more" button).
// Throttled so a batch insertion of many files runs the re-scan once.
const onFileBoxesMutation = throttle(200, () => {
  if (appliedExtensions.value) {
    applyFilterToFileBoxes(new Set(appliedExtensions.value));
  }
  if (menuVisible.value) {
    scanExtensions();
  }
});

onMounted(() => {
  el.addEventListener('keydown', onKeyDown);

  const fileBoxesContainer = document.querySelector('#diff-file-boxes');
  if (fileBoxesContainer) {
    mutationObserver = new MutationObserver(onFileBoxesMutation);
    mutationObserver.observe(fileBoxesContainer, {childList: true, subtree: false});
  }
});

onUnmounted(() => {
  document.removeEventListener('click', onOutsideClick, true);
  el.removeEventListener('keydown', onKeyDown);
  mutationObserver?.disconnect();
});
</script>
<template>
  <div class="ui scrolling dropdown custom diff-file-extension-filter">
    <button
      class="ui tiny basic button"
      :class="{'diff-ext-filter-btn-active': isFiltering}"
      @click="toggleMenu()"
      :data-tooltip-content="locale.filter_by_file_extension"
      aria-haspopup="true"
      :aria-expanded="menuVisible ? 'true' : 'false'"
      :aria-label="locale.filter_by_file_extension"
      :aria-controls="uniqueIdMenu"
    >
      <SvgIcon name="octicon-filter"/>
    </button>
    <!-- this dropdown is not managed by Fomantic UI, so it needs some classes like "transition" explicitly -->
    <div class="left menu transition" :id="uniqueIdMenu" :class="{visible: menuVisible}" v-show="menuVisible" role="menu">
      <div class="ui small input tw-mb-2 tw-w-full">
        <input
          type="text"
          v-model="searchQuery"
          class="diff-ext-search-input"
          :placeholder="locale.search"
          @keydown.escape="toggleMenu()"
        >
      </div>
      <div class="tw-flex tw-items-center tw-justify-center tw-gap-4 tw-px-2 tw-py-1">
        <button type="button" class="diff-ext-text-btn" tabindex="-1" role="menuitem" @click="selectAll()">{{ locale.select_all }}</button>
        <button type="button" class="diff-ext-text-btn" tabindex="-1" role="menuitem" @click="deselectAll()">{{ locale.deselect_all }}</button>
      </div>
      <div class="ui divider tw-my-2"/>
      <div class="ui form">
        <div class="grouped fields">
          <template v-for="ext in filteredExtensions" :key="ext.ext">
            <div class="field" role="menuitem" tabindex="-1">
              <div class="ui checkbox">
                <input
                  type="checkbox"
                  :id="`ext-filter-${ext.ext}`"
                  v-model="ext.checked"
                  @change="applyFilter()"
                >
                <label :for="`ext-filter-${ext.ext}`" class="tw-cursor-pointer">
                  <span>{{ ext.ext }}</span>
                  <span class="tw-text-text-light-2"> ({{ ext.count }})</span>
                </label>
              </div>
            </div>
          </template>
          <div v-if="filteredExtensions.length === 0" class="tw-py-4 tw-text-center tw-text-text-light-2">
            {{ locale.no_file_extensions_found }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
<style scoped>
  .ui.dropdown.diff-file-extension-filter .menu {
    margin-top: 0.25em;
    overflow-x: hidden;
    max-height: 450px;
    padding: 0.5rem;
  }

  .ui.dropdown.diff-file-extension-filter .menu .ui.input {
    display: block;
    margin: 0;
  }

  .ui.dropdown.diff-file-extension-filter .menu .ui.form {
    margin: 0;
  }

  .ui.dropdown.diff-file-extension-filter .grouped.fields > .field {
    margin-bottom: 0.5rem;
  }

  .ui.dropdown.diff-file-extension-filter .grouped.fields > .field:last-child {
    margin-bottom: 0;
  }

  .ui.dropdown.diff-file-extension-filter .diff-ext-filter-btn-active {
    outline: 1px solid var(--color-primary);
    outline-offset: -2px;
  }

  .ui.dropdown.diff-file-extension-filter .diff-ext-text-btn {
    background: none;
    border: none;
    padding: 0;
    color: var(--color-primary);
    cursor: pointer;
    font-size: inherit;
    text-align: center;
  }

  .ui.dropdown.diff-file-extension-filter .diff-ext-text-btn:hover {
    text-decoration: underline;
  }

  .ui.dropdown.diff-file-extension-filter .diff-ext-search-input {
    width: 100%;
  }
</style>
