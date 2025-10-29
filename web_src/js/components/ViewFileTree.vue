<script lang="ts" setup>
import ViewFileTreeItem from './ViewFileTreeItem.vue';
import {onMounted, useTemplateRef} from 'vue';
import {createViewFileTreeStore} from './ViewFileTreeStore.ts';

const elRoot = useTemplateRef('elRoot');

const props = defineProps({
  repoLink: {type: String, required: true},
  treePath: {type: String, required: true},
  currentRefNameSubURL: {type: String, required: true},
});

const store = createViewFileTreeStore(props);
onMounted(async () => {
  store.rootFiles = await store.loadChildren('', props.treePath);
  elRoot.value.closest('.is-loading')?.classList?.remove('is-loading');
  window.addEventListener('popstate', (e) => {
    store.selectedItem = e.state?.treePath || '';
    if (e.state?.url) store.loadViewContent(e.state.url);
  });
});
</script>

<template>
  <div class="view-file-tree-items" ref="elRoot">
    <ViewFileTreeItem v-for="item in store.rootFiles" :key="item.name" :item="item" :store="store"/>
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
