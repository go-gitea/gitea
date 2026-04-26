<script lang="ts" setup>
import {computed, onMounted, onUnmounted, useTemplateRef} from 'vue';
import type {Instance} from 'tippy.js';
import {SvgIcon} from '../svg.ts';
import {createTippy} from '../modules/tippy.ts';
import {diffTreeStore, getDiffTreeExtensionStats} from '../modules/diff-file.ts';

const props = defineProps<{
  locale: {
    filter_by_file_extension: string,
    file_extensions: string,
    select_all: string,
    deselect_all: string,
    no_file_extension: string,
  },
}>();

const store = diffTreeStore();
const triggerEl = useTemplateRef<HTMLButtonElement>('triggerEl');
const panelEl = useTemplateRef<HTMLDivElement>('panelEl');
let tippyInstance: Instance | null = null;

function displayName(ext: string): string {
  return ext || props.locale.no_file_extension;
}

const allExtensions = computed(() => getDiffTreeExtensionStats(store));

const isFiltering = computed(() => store.activeExtensions !== null);

function isChecked(ext: string): boolean {
  return store.activeExtensions === null || store.activeExtensions.includes(ext);
}

function setChecked(ext: string, checked: boolean) {
  const all = allExtensions.value.map((e) => e.ext);
  const current = new Set(store.activeExtensions ?? all);
  if (checked) current.add(ext); else current.delete(ext);
  store.activeExtensions = current.size === all.length ? null : Array.from(current);
}

const allSelected = computed(() => store.activeExtensions === null);

function toggleAll(event: MouseEvent) {
  store.activeExtensions = allSelected.value ? [] : null;
  const btn = event.currentTarget as HTMLElement & {_tippy?: Instance};
  const newLabel = allSelected.value ? props.locale.deselect_all : props.locale.select_all;
  // wait for the data-tooltip-content attribute observer to attach/update tippy
  setTimeout(() => {
    btn._tippy?.setContent(newLabel);
    btn._tippy?.show();
  }, 0);
}

onMounted(() => {
  tippyInstance = createTippy(triggerEl.value!, {
    content: panelEl.value!,
    trigger: 'click',
    interactive: true,
    hideOnClick: true,
    placement: 'bottom-end',
    theme: 'menu',
    arrow: false,
  });
});

onUnmounted(() => {
  tippyInstance?.destroy();
});
</script>

<template>
  <button
    ref="triggerEl"
    type="button"
    class="diff-ext-filter-trigger"
    :class="{'diff-ext-filter-active': isFiltering}"
    :aria-label="locale.filter_by_file_extension"
    aria-haspopup="true"
  >
    <SvgIcon name="octicon-filter"/>
  </button>
  <div ref="panelEl" class="tippy-target">
    <div class="diff-ext-filter-panel">
      <div class="diff-ext-filter-header">
        <span>{{ locale.file_extensions }}</span>
        <button type="button" class="diff-ext-icon-btn interact-bg" @click="toggleAll" :data-tooltip-content="allSelected ? locale.deselect_all : locale.select_all" :aria-label="allSelected ? locale.deselect_all : locale.select_all">
          <SvgIcon :name="allSelected ? 'octicon-checkbox' : 'gitea-empty-checkbox'"/>
        </button>
      </div>
      <div class="diff-ext-filter-list">
        <label v-for="ext in allExtensions" :key="ext.ext" class="diff-ext-filter-item">
          <input type="checkbox" :checked="isChecked(ext.ext)" @change="setChecked(ext.ext, ($event.target as HTMLInputElement).checked)">
          <span class="gt-ellipsis">{{ displayName(ext.ext) }}</span>
          <span class="diff-ext-filter-count">{{ ext.count }}</span>
        </label>
      </div>
    </div>
  </div>
</template>

<style scoped>
.diff-ext-filter-panel {
  display: flex;
  flex-direction: column;
  min-width: 220px;
  max-height: 80vh;
}

.diff-ext-filter-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  font-weight: var(--font-weight-medium);
  padding: 8px 12px;
}

.diff-ext-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
  border: none;
  color: var(--color-text);
  cursor: pointer;
  border-radius: 4px;
}

.diff-ext-filter-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow-y: auto;
  padding: 8px;
}

.diff-ext-filter-item {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
}

.diff-ext-filter-item:hover {
  background: var(--color-hover);
}

.diff-ext-filter-count {
  margin-left: auto;
  color: var(--color-text-light-2);
}

.diff-ext-filter-trigger {
  height: 32px;
  width: 32px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-secondary);
  border-radius: var(--border-radius);
  background: var(--color-button);
  color: var(--color-text);
  cursor: pointer;
}

.diff-ext-filter-trigger:hover {
  background: var(--color-hover);
}

.diff-ext-filter-active {
  border-color: var(--color-primary);
  color: var(--color-primary);
}
</style>
