<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import FileTreeItem from './FileTreeItem.vue';

defineProps<{
  id: string,
  files: any[],
  selected?: string,
  isLoading: boolean,
  collapsed: boolean,
  fileUrlGetter: any,
  fileClassGetter?: any,
  loadChildren?: any
}>();
</script>

<template>
  <div v-if="isLoading" class="file-tree-loading">
    <SvgIcon name="octicon-sync" class="job-status-rotate"/>
  </div>
  <div v-else class="file-tree-items">
    <FileTreeItem
      v-for="item in files"
      :key="item.name"
      :item="item"
      :selected="selected"
      :collapsed="collapsed"
      :load-children="loadChildren"
      :file-class-getter="fileClassGetter"
      :file-url-getter="fileUrlGetter"
    >
      <template #file-item-suffix="{item: treeItem}">
        <slot name="file-item-suffix" :item="treeItem"/>
      </template>
    </FileTreeItem>
    <slot name="footer"/>
  </div>
</template>

<style scoped>
.file-tree-loading {
  margin-top: 8px;
  text-align: center;
}
.file-tree-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-top: 8px;
  margin-right: .5rem;
}
</style>
