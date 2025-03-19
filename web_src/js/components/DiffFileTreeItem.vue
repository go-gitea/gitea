<script lang="ts" setup>
import {SvgIcon, type SvgName} from '../svg.ts';
import {diffTreeStore} from '../modules/stores.ts';
import {ref} from 'vue';
import type {Item, FileStatus} from '../utils/filetree.ts';

defineProps<{
  item: Item,
}>();

const {pageData} = window.config;
const store = diffTreeStore();
const collapsed = ref(false);

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
</script>

<template>
  <!--title instead of tooltip above as the tooltip needs too much work with the current methods, i.e. not being loaded or staying open for "too long"-->
  <a
    v-if="item.isFile" class="item-file"
    :class="{ 'selected': store.selectedItem === '#diff-' + item.file.NameHash, 'viewed': item.file.IsViewed }"
    :title="item.name" :href="'#diff-' + item.file.NameHash"
  >
    <!-- file -->
    <span class="item-icon" v-html="item.file.FileIcon"/>
    <span class="gt-ellipsis tw-flex-1">{{ item.name }}</span>
    <SvgIcon
      :name="getIconForDiffStatus(item.file.Status).name"
      :class="getIconForDiffStatus(item.file.Status).classes"
    />
  </a>

  <template v-else-if="item.isFile === false">
    <div class="item-directory" :title="item.name" @click.stop="collapsed = !collapsed">
      <!-- directory -->
      <SvgIcon :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'"/>
      <span v-if="collapsed" class="item-icon" v-html="pageData.folderIcon"/>
      <span v-else class="item-icon" v-html="pageData.openFolderIcon"/>
      <span class="gt-ellipsis">{{ item.name }}</span>
    </div>

    <div v-show="!collapsed" class="sub-items">
      <DiffFileTreeItem v-for="childItem in item.children" :key="childItem.name" :item="childItem"/>
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

.item-file.viewed {
  color: var(--color-text-light-3);
}

.item-directory {
  user-select: none;
}

.item-directory .item-icon {
  display: contents;
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
