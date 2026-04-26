<script lang="ts" setup>
import {computed, onMounted, onUnmounted, ref, useTemplateRef} from 'vue';
import type {Instance} from 'tippy.js';
import {SvgIcon} from '../svg.ts';
import {createTippy} from '../modules/tippy.ts';
import {diffTreeStore, getDiffTreeExtensionStats} from '../modules/diff-file.ts';

const props = defineProps<{
  locale: {
    filter_by_file_extension: string,
    select_all: string,
    deselect_all: string,
    search: string,
    no_file_extension: string,
    no_file_extensions_found: string,
  },
}>();

const store = diffTreeStore();
const triggerEl = useTemplateRef<HTMLButtonElement>('triggerEl');
const panelEl = useTemplateRef<HTMLDivElement>('panelEl');
const searchEl = useTemplateRef<HTMLInputElement>('searchEl');
const searchQuery = ref('');
let tippyInstance: Instance | null = null;

function displayName(ext: string): string {
  return ext || props.locale.no_file_extension;
}

const allExtensions = computed(() => getDiffTreeExtensionStats(store));

const filteredExtensions = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return allExtensions.value;
  return allExtensions.value.filter((e) => displayName(e.ext).toLowerCase().includes(q));
});

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

function selectAll() {
  store.activeExtensions = null;
}

function deselectAll() {
  store.activeExtensions = [];
}

onMounted(() => {
  tippyInstance = createTippy(triggerEl.value!, {
    content: panelEl.value!,
    trigger: 'click',
    interactive: true,
    hideOnClick: true,
    placement: 'bottom-end',
    theme: 'menu',
    onShow() {
      searchQuery.value = '';
      setTimeout(() => searchEl.value?.focus(), 0);
    },
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
    class="ui tiny basic button diff-ext-filter-trigger"
    :class="{'diff-ext-filter-active': isFiltering}"
    :data-tooltip-content="locale.filter_by_file_extension"
    :aria-label="locale.filter_by_file_extension"
    aria-haspopup="true"
  >
    <SvgIcon name="octicon-filter"/>
  </button>
  <div ref="panelEl" class="diff-ext-filter-panel tw-hidden">
    <div class="diff-ext-filter-header">{{ locale.filter_by_file_extension }}</div>
    <input
      ref="searchEl"
      v-model="searchQuery"
      type="text"
      class="diff-ext-filter-search"
      :placeholder="locale.search"
    >
    <div class="diff-ext-filter-list">
      <label v-for="ext in filteredExtensions" :key="ext.ext" class="diff-ext-filter-item">
        <input type="checkbox" :checked="isChecked(ext.ext)" @change="setChecked(ext.ext, ($event.target as HTMLInputElement).checked)">
        <span class="gt-ellipsis">{{ displayName(ext.ext) }}</span>
        <span class="diff-ext-filter-count">{{ ext.count }}</span>
      </label>
      <div v-if="filteredExtensions.length === 0" class="diff-ext-filter-empty">
        {{ locale.no_file_extensions_found }}
      </div>
    </div>
    <div class="diff-ext-filter-actions">
      <button type="button" class="diff-ext-text-btn" @click="selectAll()">{{ locale.select_all }}</button>
      <button type="button" class="diff-ext-text-btn" @click="deselectAll()">{{ locale.deselect_all }}</button>
    </div>
  </div>
</template>

<style scoped>
.diff-ext-filter-panel {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 220px;
  max-height: 450px;
  padding: 0.5rem;
}

.diff-ext-filter-header {
  font-weight: var(--font-weight-medium);
}

.diff-ext-filter-search {
  width: 100%;
  padding: 0.375rem 0.5rem;
  border: 1px solid var(--color-secondary);
  border-radius: var(--border-radius);
  background: var(--color-input-background);
  color: var(--color-text);
}

.diff-ext-filter-search:focus {
  outline: none;
  border-color: var(--color-primary);
}

.diff-ext-filter-list {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  overflow-y: auto;
}

.diff-ext-filter-item {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.2rem 0.25rem;
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

.diff-ext-filter-empty {
  text-align: center;
  color: var(--color-text-light-2);
  padding: 1rem 0;
}

.diff-ext-filter-actions {
  display: flex;
  justify-content: center;
  gap: 1rem;
  border-top: 1px solid var(--color-secondary);
  padding-top: 0.5rem;
}

.diff-ext-text-btn {
  background: none;
  border: none;
  padding: 0;
  color: var(--color-primary);
  cursor: pointer;
  font: inherit;
}

.diff-ext-text-btn:hover {
  text-decoration: underline;
}

.diff-ext-filter-active {
  outline: 1px solid var(--color-primary);
  outline-offset: -2px;
}
</style>
