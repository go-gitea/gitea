<script lang="ts" setup>
import {SvgIcon, type SvgName} from '../svg.ts';
import {shallowRef} from 'vue';
import {type DiffStatus, type DiffTreeEntry, diffTreeStore} from '../modules/diff-file.ts';

const props = defineProps<{
  item: DiffTreeEntry,
}>();

const store = diffTreeStore();
const collapsed = shallowRef(props.item.IsViewed);

function getIconForDiffStatus(pType: DiffStatus) {
  const diffTypes: Record<DiffStatus, { name: SvgName, classes: Array<string> }> = {
    '': {name: 'octicon-blocked', classes: ['text', 'red']}, // unknown case
    'added': {name: 'octicon-diff-added', classes: ['text', 'green']},
    'modified': {name: 'octicon-diff-modified', classes: ['text', 'yellow']},
    'deleted': {name: 'octicon-diff-removed', classes: ['text', 'red']},
    'renamed': {name: 'octicon-diff-renamed', classes: ['text', 'teal']},
    'copied': {name: 'octicon-diff-renamed', classes: ['text', 'green']},
    'typechange': {name: 'octicon-diff-modified', classes: ['text', 'green']}, // there is no octicon for copied, so renamed should be ok
  };
  return diffTypes[pType] ?? diffTypes[''];
}
</script>

<template>
  <template v-if="item.EntryMode === 'tree'">
    <div class="item-directory" :class="{ 'viewed': item.IsViewed }" :title="item.DisplayName" @click.stop="collapsed = !collapsed">
      <!-- directory -->
      <SvgIcon :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'"/>
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span class="tw-contents" v-html="collapsed ? store.folderIcon : store.folderOpenIcon"/>
      <span class="gt-ellipsis">{{ item.DisplayName }}</span>
    </div>

    <div v-show="!collapsed" class="sub-items">
      <DiffFileTreeItem v-for="childItem in item.Children" :key="childItem.DisplayName" :item="childItem"/>
    </div>
  </template>
  <a
    v-else
    class="item-file" :class="{ 'selected': store.selectedItem === '#diff-' + item.NameHash, 'viewed': item.IsViewed }"
    :title="item.DisplayName" :href="'#diff-' + item.NameHash"
  >
    <!-- file -->
    <!-- eslint-disable-next-line vue/no-v-html -->
    <span class="tw-contents" v-html="item.FileIcon"/>
    <span class="gt-ellipsis tw-flex-1">{{ item.DisplayName }}</span>
    <SvgIcon
      :name="getIconForDiffStatus(item.DiffStatus).name"
      :class="getIconForDiffStatus(item.DiffStatus).classes"
    />
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
