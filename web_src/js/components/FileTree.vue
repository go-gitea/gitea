<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import FileTreeItem from './FileTreeItem.vue';
// import {toggleElem} from '../utils/dom.ts';
// import {setFileFolding} from '../features/file-fold.ts';

defineProps<{
  id: string,
  files: any[],
  visible: boolean,
  selected: string,
  isLoading: boolean,
  collapsed: boolean,
  fileUrlGetter: any,
  fileClassGetter?: any,
  loadChildren?: any
}>();

// function expandSelectedFile() {
//   // expand file if the selected file is folded
//   if (props.selected) {
//     const box = document.querySelector(props.selected);
//     const folded = box?.getAttribute('data-folded') === 'true';
//     if (folded) setFileFolding(box, box.querySelector('.fold-file'), false);
//   }
// }

// function toggleVisibility() {
//   updateState(!props.visible);
// }

// function updateState(visible) {
//   toggleElem(document.querySelector(`#${props.id}`), visible);
// }
</script>

<template>
  <div v-if="visible" class="file-tree-items">
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <SvgIcon v-if="isLoading" name="octicon-sync" class="job-status-rotate"/>
    <FileTreeItem
      v-else
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
.file-tree-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-right: .5rem;
}
</style>
