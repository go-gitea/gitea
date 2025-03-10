<script lang="ts" setup>
import ViewFileTreeItem from './ViewFileTreeItem.vue';

defineProps<{
  files: any,
  selectedItem: any,
  loadChildren: any,
  loadContent: any;
}>();
</script>

<template>
  <div class="view-file-tree-items">
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <ViewFileTreeItem v-for="item in files" :key="item.name" :item="item" :selected-item="selectedItem" :load-content="loadContent" :load-children="loadChildren"/>
  </div>
</template>

<style scoped>
.view-file-tree-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-right: .5rem;
}
</style>
