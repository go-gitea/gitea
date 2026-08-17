<script lang="ts" setup>
import {computed, onMounted, onUnmounted, useTemplateRef} from 'vue';
import type {Instance} from 'tippy.js';
import {SvgIcon} from '../svg.ts';
import {createTippy} from '../modules/tippy.ts';
import {diffTreeStore, getDiffTreeExtensionStats, type DiffExtensionFilterLocale} from '../modules/diff-file.ts';

const props = defineProps<{locale: DiffExtensionFilterLocale}>();

const store = diffTreeStore();
const triggerEl = useTemplateRef<HTMLButtonElement>('triggerEl');
const panelEl = useTemplateRef<HTMLDivElement>('panelEl');
let tippyInstance: Instance;

const allExtensions = computed(() => getDiffTreeExtensionStats(store));
const isFiltering = computed(() => store.activeExtensions !== 'all');

const allCheckboxProps = computed(() => ({
  checked: store.activeExtensions === 'all',
  indeterminate: store.activeExtensions !== 'all' && store.activeExtensions.length > 0,
}));

function isChecked(ext: string): boolean {
  return store.activeExtensions === 'all' || store.activeExtensions.includes(ext);
}

function toggleExt(ext: string) {
  const all = allExtensions.value.map((e) => e.ext);
  const next = new Set(store.activeExtensions === 'all' ? all : store.activeExtensions);
  if (next.has(ext)) next.delete(ext); else next.add(ext);
  store.activeExtensions = next.size === all.length ? 'all' : Array.from(next);
}

function toggleAll() {
  store.activeExtensions = store.activeExtensions === 'all' ? [] : 'all';
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
  tippyInstance.destroy();
});
</script>

<template>
  <button
    ref="triggerEl"
    type="button"
    class="diff-ext-filter-trigger"
    :class="{'indicator-dot': isFiltering}"
    :aria-label="props.locale.filterByFileExtension"
    aria-haspopup="true"
  >
    <SvgIcon name="octicon-filter"/>
  </button>
  <div ref="panelEl" class="tippy-target">
    <div class="diff-ext-filter-menu" role="group" :aria-label="props.locale.fileExtensions">
      <div class="diff-ext-filter-header">{{ props.locale.fileExtensions }}</div>
      <div class="diff-ext-filter-list">
        <label v-for="ext in allExtensions" :key="ext.ext" class="item">
          <input type="checkbox" :checked="isChecked(ext.ext)" @change="toggleExt(ext.ext)">
          <span class="gt-ellipsis">{{ ext.ext || props.locale.noFileExtension }}</span>
          <span class="diff-ext-filter-count">{{ ext.count }}</span>
        </label>
      </div>
      <div class="divider"/>
      <label class="item">
        <input type="checkbox" v-bind.prop="allCheckboxProps" @change="toggleAll">
        <span class="gt-ellipsis">{{ props.locale.allFileExtensions }}</span>
      </label>
    </div>
  </div>
</template>

<style scoped>
.diff-ext-filter-menu {
  min-width: 220px;
}

.diff-ext-filter-header {
  padding: 6px 18px;
  font-weight: var(--font-weight-medium);
  color: var(--color-text-light-2);
  font-size: 0.875em;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.diff-ext-filter-list {
  max-height: 60vh;
  overflow-y: auto;
}

.diff-ext-filter-menu .item {
  cursor: pointer;
}

.diff-ext-filter-menu .item:has(:focus-visible) {
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
</style>
