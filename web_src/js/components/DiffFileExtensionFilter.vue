<script lang="ts" setup>
import {computed, onMounted, onUnmounted, useTemplateRef} from 'vue';
import type {Instance} from 'tippy.js';
import {SvgIcon} from '../svg.ts';
import {createTippy} from '../modules/tippy.ts';
import {diffTreeStore, getDiffTreeExtensionStats} from '../modules/diff-file.ts';

const treeEl = document.querySelector('#diff-file-tree')!;
const locale = {
  filter_by_file_extension: treeEl.getAttribute('data-filter-by-file-extension')!,
  file_extensions: treeEl.getAttribute('data-file-extensions')!,
  no_file_extension: treeEl.getAttribute('data-no-file-extension')!,
  select_all: treeEl.getAttribute('data-select-all-file-extensions')!,
  select_none: treeEl.getAttribute('data-select-none-file-extensions')!,
};

const store = diffTreeStore();
const triggerEl = useTemplateRef<HTMLButtonElement>('triggerEl');
const panelEl = useTemplateRef<HTMLDivElement>('panelEl');
let tippyInstance: Instance;

const allExtensions = computed(() => getDiffTreeExtensionStats(store));
const isFiltering = computed(() => store.activeExtensions !== 'all');

function isChecked(ext: string): boolean {
  return store.activeExtensions === 'all' || store.activeExtensions.includes(ext);
}

function toggleExt(ext: string) {
  const all = allExtensions.value.map((e) => e.ext);
  const next = new Set(store.activeExtensions === 'all' ? all : store.activeExtensions);
  if (next.has(ext)) next.delete(ext); else next.add(ext);
  store.activeExtensions = next.size === all.length ? 'all' : Array.from(next);
}

function selectAll() {
  store.activeExtensions = 'all';
}

function selectNone() {
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
    :aria-label="locale.filter_by_file_extension"
    aria-haspopup="true"
  >
    <SvgIcon name="octicon-filter"/>
  </button>
  <div ref="panelEl" class="tippy-target">
    <div class="diff-ext-filter-menu">
      <div class="diff-ext-filter-header">{{ locale.file_extensions }}</div>
      <div class="diff-ext-filter-list">
        <button
          v-for="ext in allExtensions"
          :key="ext.ext"
          type="button"
          class="item"
          role="menuitemcheckbox"
          :aria-checked="isChecked(ext.ext)"
          @click="toggleExt(ext.ext)"
        >
          <span class="diff-ext-filter-check">
            <SvgIcon v-if="isChecked(ext.ext)" name="octicon-check" :size="14"/>
          </span>
          <span class="gt-ellipsis">{{ ext.ext || locale.no_file_extension }}</span>
          <span class="diff-ext-filter-count">{{ ext.count }}</span>
        </button>
      </div>
      <div class="divider"/>
      <button type="button" class="item" role="menuitem" @click="selectAll">{{ locale.select_all }}</button>
      <button type="button" class="item" role="menuitem" @click="selectNone">{{ locale.select_none }}</button>
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

.diff-ext-filter-check {
  width: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
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
