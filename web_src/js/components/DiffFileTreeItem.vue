<script lang="ts" setup>
import SvgIcon from './SvgIcon.vue';
import type {SvgName} from '../svg.ts';
import {computed, shallowRef} from 'vue';
import {type DiffStatus, type DiffTreeEntry, diffTreeStore} from '../modules/diff-file.ts';
import {joinPath} from '../utils.ts';

const props = defineProps<{
  item: DiffTreeEntry,
  path: string,
}>();

const store = diffTreeStore();
const collapsed = shallowRef(props.item.IsViewed);
const boxId = computed(() => store.boxIdMap.get(props.path));

const diffStatusIcons: Record<DiffStatus, {name: SvgName, class: string}> = {
  '': {name: 'octicon-blocked', class: 'tw-text-red'},
  'added': {name: 'octicon-diff-added', class: 'tw-text-green'},
  'modified': {name: 'octicon-diff-modified', class: 'tw-text-yellow'},
  'deleted': {name: 'octicon-diff-removed', class: 'tw-text-red'},
  'renamed': {name: 'octicon-diff-renamed', class: 'tw-text-teal'},
  'copied': {name: 'octicon-diff-renamed', class: 'tw-text-green'}, // there is no octicon for copied, so renamed should be ok
  'typechanged': {name: 'octicon-diff-modified', class: 'tw-text-green'},
  'unmerged': {name: 'octicon-blocked', class: 'tw-text-red'},
  'unknown': {name: 'octicon-blocked', class: 'tw-text-red'},
};
</script>

<template>
  <template v-if="item.Children">
    <div class="item-directory" :class="{ 'viewed': item.IsViewed }" :title="item.Name" @click.stop="collapsed = !collapsed">
      <!-- directory -->
      <SvgIcon :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'"/>
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span class="tw-contents" v-html="collapsed ? store.FolderIcon : store.FolderOpenIcon"/>
      <span class="gt-ellipsis">{{ item.Name }}</span>
    </div>

    <div v-show="!collapsed" class="sub-items">
      <DiffFileTreeItem v-for="childItem in item.Children!" :key="childItem.Name" :item="childItem" :path="joinPath(path, childItem.Name)"/>
    </div>
  </template>
  <a
    v-else
    class="item-file" :class="{ 'selected': Boolean(boxId) && store.selectedItem === `#${boxId}`, 'viewed': item.IsViewed }"
    :title="item.Name" :href="boxId ? `#${boxId}` : undefined"
  >
    <svg :class="item.IconClass ?? store.FileIconClass" width="16" height="16" aria-hidden="true">
      <use :href="`#${item.Icon}`"/>
    </svg>
    <span class="gt-ellipsis tw-flex-1">{{ item.Name }}</span>
    <SvgIcon v-bind="diffStatusIcons[item.DiffStatus!] ?? diffStatusIcons['']"/>
  </a>
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
