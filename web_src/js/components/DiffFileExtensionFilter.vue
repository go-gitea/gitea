<script lang="ts" setup>
import {computed, onMounted, onUnmounted, useTemplateRef} from 'vue';
import type {Instance} from 'tippy.js';
import SvgIcon from './SvgIcon.vue';
import type {SvgName} from '../svg.ts';
import {createTippy} from '../modules/tippy.ts';
import {diffTreeStore, extDotfile, getDiffTreeExtensionStats, type DiffExtensionFilterLocale} from '../modules/diff-file.ts';

const props = defineProps<{locale: DiffExtensionFilterLocale}>();

const store = diffTreeStore();
const triggerEl = useTemplateRef<HTMLButtonElement>('triggerEl');
const panelEl = useTemplateRef<HTMLDivElement>('panelEl');
let tippyInstance: Instance;

const allExtensions = computed(() => getDiffTreeExtensionStats(store));
const isFiltering = computed(() => store.activeExtensions !== 'all' || Boolean(store.filenameFilterQuery));
const allIcon = computed<SvgName | null>(() => {
  if (store.activeExtensions === 'all') return 'octicon-check';
  return store.activeExtensions.length ? 'octicon-dash' : null;
});

function isChecked(ext: string): boolean {
  return store.activeExtensions === 'all' || store.activeExtensions.includes(ext);
}

function extLabel(ext: string): string {
  if (ext === extDotfile) return props.locale.dotfileExtension;
  return ext || props.locale.noFileExtension;
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

function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape') tippyInstance.hide();
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
    limitSizeToViewport: {vertical: true},
    onShow: () => document.addEventListener('keydown', onKeyDown),
    onHide: () => document.removeEventListener('keydown', onKeyDown),
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
  >
    <SvgIcon name="octicon-filter"/>
  </button>
  <div ref="panelEl" class="tippy-target">
    <div class="diff-ext-filter-menu" role="menu" :aria-label="props.locale.fileExtensions">
      <div class="diff-ext-filter-header">{{ props.locale.fileExtensions }}</div>
      <div class="diff-ext-filter-list">
        <button
          v-for="ext in allExtensions" :key="ext.ext"
          type="button" class="item" role="menuitemcheckbox"
          :aria-checked="isChecked(ext.ext)" @click="toggleExt(ext.ext)"
        >
          <span class="diff-ext-filter-check">
            <SvgIcon v-if="isChecked(ext.ext)" name="octicon-check"/>
          </span>
          <span class="gt-ellipsis">{{ extLabel(ext.ext) }}</span>
          <span class="diff-ext-filter-count">{{ ext.count }}</span>
        </button>
      </div>
      <div class="divider"/>
      <button
        type="button" class="item" role="menuitemcheckbox"
        :aria-checked="store.activeExtensions === 'all'" @click="toggleAll"
      >
        <span class="diff-ext-filter-check">
          <SvgIcon v-if="allIcon" :name="allIcon"/>
        </span>
        <span class="gt-ellipsis">{{ props.locale.allFileExtensions }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.diff-ext-filter-menu {
  min-width: 192px;
  max-width: 320px;
}

.diff-ext-filter-header {
  padding: 6px 16px;
  color: var(--color-text-light-2);
  font-size: 12px;
  font-weight: var(--font-weight-semibold);
}

.diff-ext-filter-list {
  display: flex;
  flex-direction: column;
}

.diff-ext-filter-menu .item {
  width: auto; /* buttons are shrink-to-fit, the flex column parent stretches them */
  margin: 0 4px; /* matches the menu's vertical padding so the inset is even on all sides */
  padding: 6px 12px;
  gap: 8px;
  border: none;
  border-radius: var(--border-radius-medium);
  font: inherit;
  text-align: left;
}

.diff-ext-filter-check {
  display: flex;
  flex: 0 0 16px;
  color: var(--color-text-light-2);
}

.diff-ext-filter-count {
  margin-left: auto;
  padding: 2px 6px;
  border-radius: var(--border-radius-full);
  background: var(--color-label-bg);
  font-size: 12px;
  font-weight: var(--font-weight-semibold);
  line-height: 12px;
}

.diff-ext-filter-trigger {
  height: 32px;
  width: 32px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-secondary);
  border-radius: var(--border-radius-medium);
  background: var(--color-button);
  color: var(--color-text-light-2);
}

.diff-ext-filter-trigger:hover {
  background: var(--color-hover);
}
</style>
