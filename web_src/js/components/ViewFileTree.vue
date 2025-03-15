<script lang="ts" setup>
import ViewFileTreeItem from './ViewFileTreeItem.vue';
import {onMounted, ref} from 'vue';
import {pathEscapeSegments} from '../utils/url.ts';
import {GET} from '../modules/fetch.ts';

const elRoot = ref<HTMLElement | null>(null);

const props = defineProps({
  repoLink: {type: String, required: true},
  treePath: {type: String, required: true},
  currentRefNameSubURL: {type: String, required: true},
});

const files = ref([]);
const selectedItem = ref('');

async function loadChildren(treePath: string, subPath: string = '') {
  const response = await GET(`${props.repoLink}/tree-view/${props.currentRefNameSubURL}/${pathEscapeSegments(treePath)}?sub_path=${encodeURIComponent(subPath)}`);
  const json = await response.json();
  return json.fileTreeNodes ?? null;
}

async function loadViewContent(treePath: string) {
  // load content by path (content based on home_content.tmpl)
  window.history.pushState({treePath}, null, `${props.repoLink}/src/${props.currentRefNameSubURL}/${pathEscapeSegments(treePath)}`);
  const response = await GET(`${window.location.href}?only_content=true`);
  const contentEl = document.querySelector('.repo-view-content');
  contentEl.innerHTML = await response.text();
}

onMounted(async () => {
  selectedItem.value = props.treePath;
  files.value = await loadChildren('', props.treePath);
  elRoot.value.closest('.is-loading')?.classList?.remove('is-loading');
  window.addEventListener('popstate', (e) => {
    selectedItem.value = e.state?.treePath || '';
    loadViewContent(selectedItem.value);
  });
});
</script>

<template>
  <div class="view-file-tree-items" ref="elRoot">
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <ViewFileTreeItem v-for="item in files" :key="item.name" :item="item" :selected-item="selectedItem" :load-content="loadViewContent" :load-children="loadChildren"/>
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
