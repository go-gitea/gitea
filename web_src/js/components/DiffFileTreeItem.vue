<script lang="ts" setup>
import {SvgIcon, type SvgName} from '../svg.ts';
import {diffTreeStore} from '../modules/stores.ts';
import {computed, nextTick, ref, watch} from 'vue';
import type {Item, DirItem, File, FileStatus} from '../utils/filetree.ts';

const props = defineProps<{
  item: Item,
  setViewed?:(val: boolean) => void,
}>();

const count = ref(0);
let pendingUpdate = 0;
let pendingTimer: Promise<void> | undefined;

const setCount = (isViewed: boolean) => {
  pendingUpdate += (isViewed ? 1 : -1);

  if (pendingTimer === undefined) {
    pendingTimer = nextTick(() => {
      count.value = Math.max(0, count.value + pendingUpdate);
      pendingUpdate = 0;
      pendingTimer = undefined;
    });
  }
};

const isViewed = computed(() => {
  return props.item.isFile ? props.item.file.IsViewed : (props.item as DirItem).children.length === count.value;
});

watch(
  () => isViewed.value,
  (newVal) => {
    if (props.setViewed) {
      props.setViewed(newVal);
    }
  },
  {immediate: true, flush: 'post'},
);

const store = diffTreeStore();

/**
 * Behavior:
 * - Viewed folders collapse on initial load (based on `isViewed` state)
 * - Manual expand/collapse via clicks remains enabled
 */
const collapsed = ref(isViewed.value);

function getIconForDiffStatus(pType: FileStatus) {
  const diffTypes: Record<FileStatus, { name: SvgName, classes: Array<string> }> = {
    'added': {name: 'octicon-diff-added', classes: ['text', 'green']},
    'modified': {name: 'octicon-diff-modified', classes: ['text', 'yellow']},
    'deleted': {name: 'octicon-diff-removed', classes: ['text', 'red']},
    'renamed': {name: 'octicon-diff-renamed', classes: ['text', 'teal']},
    'copied': {name: 'octicon-diff-renamed', classes: ['text', 'green']},
    'typechange': {name: 'octicon-diff-modified', classes: ['text', 'green']}, // there is no octicon for copied, so renamed should be ok
  };
  return diffTypes[pType];
}

function fileIcon(file: File) {
  if (file.IsSubmodule) {
    return 'octicon-file-submodule';
  }
  return 'octicon-file';
}
</script>

<template>
  <!--title instead of tooltip above as the tooltip needs too much work with the current methods, i.e. not being loaded or staying open for "too long"-->
  <a
    v-if="item.isFile" class="item-file"
    :class="{ 'selected': store.selectedItem === '#diff-' + item.file.NameHash, 'viewed': isViewed }"
    :title="item.name" :href="'#diff-' + item.file.NameHash"
  >
    <!-- file -->
    <SvgIcon :name="fileIcon(item.file)"/>
    <span class="gt-ellipsis tw-flex-1">{{ item.name }}</span>
    <SvgIcon
      :name="getIconForDiffStatus(item.file.Status).name"
      :class="getIconForDiffStatus(item.file.Status).classes"
    />
  </a>

  <template v-else-if="item.isFile === false">
    <div class="item-directory" :class="{ 'viewed': isViewed }" :title="item.name" @click.stop="collapsed = !collapsed">
      <!-- directory -->
      <SvgIcon :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'"/>
      <SvgIcon
        class="text primary"
        :name="collapsed ? 'octicon-file-directory-fill' : 'octicon-file-directory-open-fill'"
      />
      <span class="gt-ellipsis">{{ item.name }} {{ count }}</span>
    </div>

    <div v-show="!collapsed" class="sub-items">
      <DiffFileTreeItem v-for="childItem in item.children" :key="childItem.name" :item="childItem" :setViewed="setCount"/>
    </div>
  </template>
</template>
<style scoped>
a,
a:hover {
  text-decoration: none;
  color: var(--color-text);
}

.sub-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-left: 13px;
  border-left: 1px solid var(--color-secondary);
}

.sub-items .item-file {
  padding-left: 18px;
}

.item-file.selected {
  color: var(--color-text);
  background: var(--color-active);
  border-radius: 4px;
}

.item-file.viewed,
.item-directory.viewed {
  color: var(--color-text-light-3);
}

.item-directory {
  user-select: none;
}

.item-file,
.item-directory {
  display: flex;
  align-items: center;
  gap: 0.25em;
  padding: 6px;
}

.item-file:hover,
.item-directory:hover {
  color: var(--color-text);
  background: var(--color-hover);
  border-radius: 4px;
  cursor: pointer;
}
</style>
