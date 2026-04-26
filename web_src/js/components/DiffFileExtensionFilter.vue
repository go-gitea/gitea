<script lang="ts" setup>
import {ref, computed, onMounted, onUnmounted} from 'vue';
import {SvgIcon} from '../svg.ts';
import {createTippy} from '../modules/tippy.ts';
import {diffTreeStore, getDiffFileExtension, getDiffTreeExtensionStats, hasActiveDiffTreeFilter, applyFiltersToFileBoxes} from '../modules/diff-file.ts';
import type {Instance} from 'tippy.js';

type Extension = {
  ext: string,
  checked: boolean,
  count: number,
}

const props = defineProps<{
  locale: Record<string, string>,
}>();

const store = diffTreeStore();
const btnRef = ref<HTMLButtonElement>();
const menuRef = ref<HTMLElement>();
const extensions = ref<Array<Extension>>([]);
const searchQuery = ref('');
let mutationObserver: MutationObserver | null = null;
let tippyInstance: Instance | null = null;
const isFiltering = computed(() => hasActiveDiffTreeFilter(store));

const filteredExtensions = computed(() => {
  const query = searchQuery.value.trim().toLowerCase();
  if (!query) return extensions.value;
  return extensions.value.filter((ext) => ext.ext.toLowerCase().includes(query));
});

function scanExtensions() {
  extensions.value = getDiffTreeExtensionStats(store)
    .map(({ext, count}) => ({
      ext,
      checked: store.activeExtensions ? store.activeExtensions.includes(ext) : true,
      count,
    }));
}

function applyExtensionFilter(checkedExtensions: Set<string>) {
  const allChecked = extensions.value.every((ext) => checkedExtensions.has(ext.ext));
  store.activeExtensions = allChecked ? null : Array.from(checkedExtensions);
  applyFiltersToFileBoxes(store);
}

function focusElem(elem: HTMLElement | null, prevElem: HTMLElement | null) {
  if (elem) {
    elem.tabIndex = 0;
    if (prevElem) prevElem.tabIndex = -1;
    const focusTarget = elem.querySelector<HTMLElement>('input, button') ?? elem;
    focusTarget.focus();
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
  applyExtensionFilter(checkedExtensions);
}

function onKeyDown(event: KeyboardEvent) {
  const menu = menuRef.value;
  if (!menu) return;

  const currentFocused = document.activeElement as HTMLElement;
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
      tippyInstance?.hide();
      btnRef.value?.focus();
      break;
  }
}

function toggleTippy() {
  if (tippyInstance!.state.isVisible) {
    tippyInstance!.hide();
  } else {
    tippyInstance!.show();
  }
}

onMounted(() => {
  store.noFileExtensionLabel = props.locale.no_file_extension;
  tippyInstance = createTippy(menuRef.value!, {
    content: menuRef.value!,
    trigger: 'manual',
    interactive: true,
    placement: 'bottom-start',
    theme: 'menu',
    hideOnClick: true,
    getReferenceClientRect: () => btnRef.value!.getBoundingClientRect(),
    onShow() {
      searchQuery.value = '';
      scanExtensions();
      setTimeout(() => {
        (menuRef.value?.querySelector('.diff-ext-search-input') as HTMLInputElement | null)?.focus();
      }, 0);
    },
  });
  btnRef.value!.addEventListener('click', toggleTippy);
  menuRef.value!.addEventListener('keydown', onKeyDown);

  const fileBoxesContainer = document.querySelector('#diff-file-boxes');
  if (fileBoxesContainer) {
    mutationObserver = new MutationObserver(() => {
      applyFiltersToFileBoxes(store);
      if (tippyInstance?.state.isVisible) scanExtensions();
    });
    mutationObserver.observe(fileBoxesContainer, {childList: true, subtree: false});
  }
});

onUnmounted(() => {
  tippyInstance?.destroy();
  btnRef.value?.removeEventListener('click', toggleTippy);
  menuRef.value?.removeEventListener('keydown', onKeyDown);
  mutationObserver?.disconnect();
});
</script>
<template>
  <button
    ref="btnRef"
    class="ui tiny basic button tw-flex-shrink-0"
    :class="{'diff-ext-filter-btn-active': isFiltering}"
    :data-tooltip-content="props.locale.filter_by_file_extension"
    aria-haspopup="true"
    :aria-label="props.locale.filter_by_file_extension"
  >
    <SvgIcon name="octicon-filter"/>
  </button>
  <div ref="menuRef" class="diff-extension-filter-menu">
    <div class="header">{{ props.locale.filter_by_file_extension }}</div>
    <div class="ui divider tw-mt-2 tw-mb-0"/>
    <!-- Search input -->
    <div class="ui form tw-mb-2">
      <div class="ui input fluid field tw-mb-0">
        <input
          type="text"
          v-model="searchQuery"
          class="diff-ext-search-input"
          :placeholder="props.locale.search"
        >
      </div>
    </div>
    <div class="ui divider tw-mt-2 tw-mb-0"/>
    <div class="ui form">
      <!-- Extension checkboxes -->
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
          {{ props.locale.no_file_extensions_found }}
        </div>
      </div>
    </div>

    <!-- Select all / Deselect all buttons -->
    <div class="ui divider tw-my-2"/>
    <div class="tw-flex tw-items-center tw-justify-center tw-gap-4 tw-px-2 tw-py-1">
      <button type="button" class="diff-ext-text-btn" tabindex="-1" role="menuitem" @click="selectAll()">{{ props.locale.select_all }}</button>
      <button type="button" class="diff-ext-text-btn" tabindex="-1" role="menuitem" @click="deselectAll()">{{ props.locale.deselect_all }}</button>
    </div>
  </div>
</template>
<style scoped>
  .diff-ext-filter-btn-active {
    outline: 1px solid var(--color-primary);
    outline-offset: -2px;
  }

  .diff-extension-filter-menu {
    overflow-x: hidden;
    max-height: 450px;
    overflow-y: auto;
    padding: 0.75rem;
    padding-top: 0.5rem;
    min-width: 200px;
  }

  .diff-extension-filter-menu > .header {
    margin-top: 0;
    padding-top: 0;
    color: var(--color-text);
    font-weight: bold;
  }

  .diff-extension-filter-menu .ui.form {
    margin: 0;
  }

  .diff-extension-filter-menu .grouped.fields > .field {
    margin-bottom: 0.5rem;
  }

  .diff-extension-filter-menu .grouped.fields > .field:last-child {
    margin-bottom: 0;
  }

  .diff-ext-text-btn {
    background: none;
    border: none;
    padding: 0;
    color: var(--color-primary);
    cursor: pointer;
    font-size: inherit;
    text-align: center;
  }

  .diff-ext-text-btn:hover {
    text-decoration: underline;
  }

  .diff-ext-search-input {
    width: 100%;
  }
</style>
