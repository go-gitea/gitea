<script lang="ts" setup>
import {ref, computed, onMounted, onUnmounted} from 'vue';
import {SvgIcon} from '../svg.ts';
import {generateElemId} from '../utils/dom.ts';

/**
 * Represents a file extension entry in the filter dropdown
 * @property ext - Extension with dot (e.g., ".ts", ".go") or "(no extension)"
 * @property checked - Whether this extension is currently selected for display
 * @property count - Total number of diff files with this extension
 */
type Extension = {
  ext: string,
  checked: boolean,
  count: number,
}

const el = document.querySelector<HTMLElement>('#diff-extension-filter')!;
const menuVisible = ref(false);
const extensions = ref<Array<Extension>>([]);
const isFiltering = ref(false);
const appliedExtensions = ref<Array<string> | null>(null);
const searchQuery = ref('');
const mutationObserver = ref<MutationObserver | null>(null);
const uniqueIdMenu = generateElemId('diff-extension-filter-menu-');
const locale = {
  filter_by_file_extension: el.getAttribute('data-filter_by_file_extension') ?? 'Filter by extension',
  select_all: el.getAttribute('data-select_all') ?? 'Select all',
  deselect_all: el.getAttribute('data-deselect_all') ?? 'Deselect all',
  apply: el.getAttribute('data-apply') ?? 'Apply',
  search: el.getAttribute('data-search') ?? 'Search extensions...',
} as Record<string, string>;

/**
 * Filter extensions based on search query
 * Matches against extension name (e.g., ".ts", ".go")
 */
const filteredExtensions = computed(() => {
  if (!searchQuery.value.trim()) {
    return extensions.value;
  }
  const query = searchQuery.value.toLowerCase();
  return extensions.value.filter((ext) => ext.ext.toLowerCase().includes(query));
});

/**
 * Extract file extension from filename
 * Returns the extension with dot (e.g., ".ts", ".go")
 * Returns "(no extension)" for files without extension
 */
function getExtension(filename: string): string {
  const lastDot = filename.lastIndexOf('.');
  if (lastDot === -1 || lastDot === 0) {
    return '(no extension)';
  }
  return filename.substring(lastDot);
}

/**
 * Scan all diff-file-box elements and build extension list
 * Checks current visibility state and sets checked state accordingly
 * Updates the extensions array sorted by file count (descending)
 */
function scanExtensions() {
  const extensionMap = new Map<string, {total: number, visible: number}>();
  const fileBoxes = document.querySelectorAll<HTMLElement>('#diff-file-boxes .diff-file-box[data-new-filename]');

  let hiddenCount = 0;
  fileBoxes.forEach((box) => {
    const filename = box.getAttribute('data-new-filename') || '';
    const ext = getExtension(filename);
    const isHidden = box.classList.contains('tw-hidden');
    if (!extensionMap.has(ext)) {
      extensionMap.set(ext, {total: 0, visible: 0});
    }
    const stats = extensionMap.get(ext)!;
    stats.total += 1;
    if (!isHidden) {
      stats.visible += 1;
    } else {
      hiddenCount += 1;
    }
  });

  extensions.value = Array.from(extensionMap.entries())
    .map(([ext, stats]) => ({
      ext,
      checked: appliedExtensions.value ? appliedExtensions.value.includes(ext) : stats.visible > 0,
      count: stats.total,
    }))
    .sort((a, b) => b.count - a.count);

  isFiltering.value = hiddenCount > 0;
}

/**
 * Apply filter to all diff file boxes by adding/removing tw-hidden class
 * Updates isFiltering state and persists applied extensions for load-more sync
 * @param checkedExtensions Set of extensions that should be visible
 */
function applyFilterToFileBoxes(checkedExtensions: Set<string>) {
  const fileBoxes = document.querySelectorAll<HTMLElement>('#diff-file-boxes .diff-file-box[data-new-filename]');
  let hiddenCount = 0;

  fileBoxes.forEach((box) => {
    const filename = box.getAttribute('data-new-filename') || '';
    const ext = getExtension(filename);
    const isChecked = checkedExtensions.has(ext);

    if (isChecked) {
      box.classList.remove('tw-hidden');
    } else {
      box.classList.add('tw-hidden');
      hiddenCount += 1;
    }
  });

  isFiltering.value = hiddenCount > 0;
  appliedExtensions.value = hiddenCount > 0 ? Array.from(checkedExtensions) : null;
}

/**
 * Focus a menu item element, updating tabIndex for keyboard navigation
 * Focuses the first input or button within the element if available
 * @param elem Element to focus
 * @param prevElem Previous focused element to remove from tab order
 */
function focusElem(elem: HTMLElement | null, prevElem: HTMLElement | null) {
  if (elem) {
    elem.tabIndex = 0;
    if (prevElem) prevElem.tabIndex = -1;
    // Focus the input/button inside the menuitem if it exists, otherwise focus the item itself
    const focusTarget = elem.querySelector('input, button') as HTMLElement || elem;
    focusTarget.focus();
  }
}

/**
 * Toggle dropdown menu visibility
 * Rescans extensions when opening, clears search when closing
 */
function toggleMenu() {
  menuVisible.value = !menuVisible.value;
  if (menuVisible.value) {
    searchQuery.value = '';
    scanExtensions();
    setTimeout(() => {
      const searchInput = el.querySelector('.diff-ext-search-input') as HTMLInputElement;
      if (searchInput) searchInput.focus();
    }, 0);
  }
}

/**
 * Select all file extensions
 */
function selectAll() {
  for (const ext of extensions.value) {
    ext.checked = true;
  }
}

/**
 * Deselect all file extensions
 */
function deselectAll() {
  for (const ext of extensions.value) {
    ext.checked = false;
  }
}

/**
 * Apply the current filter selection to diff files and close the dropdown
 * Hides/shows diff-file-box elements based on checked extensions
 */
function applyFilter() {
  const checkedExtensions = new Set(extensions.value.filter((e) => e.checked).map((e) => e.ext));
  applyFilterToFileBoxes(checkedExtensions);
  toggleMenu(false);
}

/**
 * Close dropdown when clicking outside the component
 * @param event Click event
 */
function onBodyClick(event: MouseEvent) {
  if (!el.contains(event.target as Node)) {
    if (menuVisible.value) {
      toggleMenu();
    }
  }
}

/**
 * Handle keyboard navigation within the dropdown menu
 * Arrow Up/Down: navigate through checkboxes and buttons
 * Space/Enter: toggle checkboxes or activate buttons
 * Escape: close the dropdown
 * @param event Keyboard event
 */
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

      // Try to find and toggle a checkbox (currentElement may be the input itself or a parent)
      const checkbox = (currentElement?.matches('input[type="checkbox"]')
        ? currentElement
        : currentElement?.querySelector('input[type="checkbox"]')) as HTMLInputElement | null;
      if (checkbox) {
        checkbox.checked = !checkbox.checked;
        checkbox.dispatchEvent(new Event('change', {bubbles: true}));
        break;
      }

      // If focused element is a button, click it
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

onMounted(() => {
  document.body.addEventListener('click', onBodyClick, true);
  el.addEventListener('keydown', onKeyDown);

  // Watch for new files being added (e.g., when "load more" is clicked)
  const fileBoxesContainer = document.querySelector('#diff-file-boxes');
  if (fileBoxesContainer) {
    mutationObserver.value = new MutationObserver(() => {
      if (appliedExtensions.value) {
        applyFilterToFileBoxes(new Set(appliedExtensions.value));
      }

      if (menuVisible.value) {
        scanExtensions();
      }
    });
    mutationObserver.value.observe(fileBoxesContainer, {childList: true, subtree: false});
  }
});

onUnmounted(() => {
  document.body.removeEventListener('click', onBodyClick, true);
  el.removeEventListener('keydown', onKeyDown);

  if (mutationObserver.value) {
    mutationObserver.value.disconnect();
  }
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
      :aria-label="locale.filter_by_file_extension"
      :aria-controls="uniqueIdMenu"
    >
      <SvgIcon name="octicon-filter"/>
    </button>
    <!-- this dropdown is not managed by Fomantic UI, so it needs some classes like "transition" explicitly -->
    <div class="left menu transition" :id="uniqueIdMenu" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak :aria-expanded="menuVisible ? 'true': 'false'">
      <div class="header">{{ locale.filter_by_file_extension }}</div>
      <div class="ui divider tw-mt-2 tw-mb-0"/>
      <!-- Search input -->
      <div class="ui form tw-mb-2">
        <div class="ui input fluid field tw-mb-0">
          <input
            type="text"
            v-model="searchQuery"
            class="diff-ext-search-input"
            :placeholder="locale.search"
            @keydown.escape="toggleMenu()"
          />
        </div>
      </div>
      <div class="ui divider tw-mt-2 tw-mb-0"/>
      <div class="ui form">
        <!-- Extension checkboxes -->
        <div class="grouped fields">
          <template v-if="filteredExtensions.length > 0" v-for="ext in filteredExtensions" :key="ext.ext">
            <div class="field" role="menuitem" tabindex="-1">
              <div class="ui checkbox">
                <input
                  type="checkbox"
                  :id="`ext-filter-${ext.ext}`"
                  v-model="ext.checked"
                >
                <label :for="`ext-filter-${ext.ext}`" class="tw-cursor-pointer">
                  <span class="tw-font-mono">{{ ext.ext }}</span>
                  <span class="tw-text-text-light-2"> ({{ ext.count }})</span>
                </label>
              </div>
            </div>
          </template>
          <div v-if="filteredExtensions.length === 0" class="tw-py-4 tw-text-center tw-text-text-light-2">
            {{ locale.no_results ?? 'No extensions found' }}
          </div>
        </div>
      </div>

      <!-- Select all / Deselect all buttons -->
      <div class="ui divider tw-my-2"/>
      <div class="tw-flex tw-items-center tw-justify-center tw-gap-4 tw-px-2 tw-py-1">
        <button type="button" class="diff-ext-text-btn" tabindex="-1" role="menuitem" @click="selectAll()">{{ locale.select_all }}</button>
        <button type="button" class="diff-ext-text-btn" tabindex="-1" role="menuitem" @click="deselectAll()">{{ locale.deselect_all }}</button>
      </div>

      <!-- Apply button -->
      <div class="ui divider tw-my-2"/>
      <button type="button" class="ui button fluid" tabindex="-1" role="menuitem" @click="applyFilter()">
        {{ locale.apply }}
      </button>
    </div>
  </div>
</template>
<style scoped>
  .ui.dropdown.diff-file-extension-filter .menu {
    margin-top: 0.25em;
    overflow-x: hidden;
    max-height: 450px;
    padding: 0.75rem;
    padding-top: 0.5rem;
  }

  .ui.dropdown.diff-file-extension-filter .menu > .header {
    margin-top: 0;
    padding-top: 0;
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
    outline-offset: -1px;
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

  .ui.dropdown.diff-file-extension-filter .ui.input {
    margin-bottom: 0.5rem;
  }
</style>
