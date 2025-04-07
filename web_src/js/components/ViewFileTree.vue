<script lang="ts" setup>
import ViewFileTreeItem from './ViewFileTreeItem.vue';
import {onMounted, ref} from 'vue';
import {pathEscapeSegments} from '../utils/url.ts';
import {GET} from '../modules/fetch.ts';
import {createElementFromHTML} from '../utils/dom.ts';

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
  const poolSvgs = [];
  for (const [svgId, svgContent] of Object.entries(json.renderedIconPool ?? {})) {
    if (!document.querySelector(`.global-svg-icon-pool #${svgId}`)) poolSvgs.push(svgContent);
  }
  if (poolSvgs.length) {
    const svgContainer = createElementFromHTML('<div class="global-svg-icon-pool tw-hidden"></div>');
    svgContainer.innerHTML = poolSvgs.join('');
    document.body.append(svgContainer);
  }
  return json.fileTreeNodes ?? null;
}

async function loadViewContent(url: string) {
  url = url.includes('?') ? url.replace('?', '?only_content=true') : `${url}?only_content=true`;
  const response = await GET(url);
  document.querySelector('.repo-view-content').innerHTML = await response.text();
}

async function navigateTreeView(treePath: string) {
  const url = `${props.repoLink}/src/${props.currentRefNameSubURL}/${pathEscapeSegments(treePath)}`;
  window.history.pushState({treePath, url}, null, url);
  selectedItem.value = treePath;
  await loadViewContent(url);
}

onMounted(async () => {
  selectedItem.value = props.treePath;
  files.value = await loadChildren('', props.treePath);
  elRoot.value.closest('.is-loading')?.classList?.remove('is-loading');
  window.addEventListener('popstate', (e) => {
    selectedItem.value = e.state?.treePath || '';
    if (e.state?.url) loadViewContent(e.state.url);
  });
});
</script>

<template>
  <div class="view-file-tree-items" ref="elRoot">
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <ViewFileTreeItem v-for="item in files" :key="item.name" :item="item" :selected-item="selectedItem" :navigate-view-content="navigateTreeView" :load-children="loadChildren"/>
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
