<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {onMounted, ref} from 'vue';

type Item = {
  entryName: string;
  entryMode: string;
  fileIcon: string;
  fullPath: string;
  submoduleUrl?: string;
  children?: Item[];
};

const {pageData} = window.config;

const props = defineProps<{
  item: Item,
  navigateViewContent:(treePath: string) => void,
  loadChildren:(treePath: string, subPath?: string) => Promise<Item[]>,
  selectedItem?: string,
}>();

const elRoot = ref<HTMLElement | null>(null);
const isLoading = ref(false);
const children = ref(props.item.children);
const collapsed = ref(!props.item.children);
const isDirectory = ref(!(
  props.item.entryMode === 'commit' ||
  props.item.entryMode === 'symlink' ||
  props.item.entryMode !== 'tree'
));

const doLoadChildren = async () => {
  collapsed.value = !collapsed.value;
  if (!collapsed.value && props.loadChildren) {
    isLoading.value = true;
    try {
      children.value = await props.loadChildren(props.item.fullPath);
    } finally {
      isLoading.value = false;
    }
  }
};

const doLoadDirContent = () => {
  doLoadChildren();
  props.navigateViewContent(props.item.fullPath);
};

const doLoadFileContent = () => {
  props.navigateViewContent(props.item.fullPath);
};

const doGotoSubModule = () => {
  location.href = props.item.submoduleUrl;
};

const doItemAction = () => {
  if (props.item.entryMode === 'symlink' || props.item.entryMode === 'tree') {
    doLoadFileContent();
  } else if (props.item.entryMode === 'commit') {
    doGotoSubModule();
  } else {
    doLoadDirContent();
  }
};

onMounted(async () => {
  elRoot.value.querySelector('.item-icon svg')?.classList?.add('preview-square');
});
</script>

<!--title instead of tooltip above as the tooltip needs too much work with the current methods, i.e. not being loaded or staying open for "too long"-->
<template>
  <div
    ref="elRoot"
    class="tree-item"
    :class="{'selected': selectedItem === item.fullPath, 'type-directory': isDirectory}"
    :title="item.entryName"
    @click.stop="doItemAction"
  >
    <div v-if="isDirectory" class="item-toggle">
      <!-- directory -->
      <SvgIcon v-if="isLoading" name="octicon-sync" class="job-status-rotate"/>
      <SvgIcon v-else :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'" @click.stop="doLoadChildren"/>
    </div>
    <div class="item-content">
      <span v-if="isDirectory && collapsed" class="item-icon" v-html="pageData.folderIcon"/>
      <span v-else-if="isDirectory && !collapsed" class="item-icon" v-html="pageData.openFolderIcon"/>
      <span v-else class="item-icon" v-html="item.fileIcon"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.entryName }}</span>
    </div>
  </div>

  <div v-if="children?.length" v-show="!collapsed" class="sub-items">
    <ViewFileTreeItem v-for="childItem in children" :key="childItem.entryName" :item="childItem" :selected-item="selectedItem" :navigate-view-content="navigateViewContent" :load-children="loadChildren"/>
  </div>
</template>
<style scoped>
.sub-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-left: 14px;
  border-left: 1px solid var(--color-secondary);
}

.tree-item.selected {
  color: var(--color-text);
  background: var(--color-active);
  border-radius: 4px;
}

.tree-item.type-directory {
  user-select: none;
}

.tree-item {
  display: grid;
  grid-template-columns: 16px 1fr;
  grid-template-areas: "toggle content";
  gap: 0.25em;
  padding: 6px;
}

.tree-item:hover {
  color: var(--color-text);
  background: var(--color-hover);
  border-radius: 4px;
  cursor: pointer;
}

.item-toggle {
  grid-area: toggle;
  display: flex;
  align-items: center;
}

.item-content {
  grid-area: content;
  display: flex;
  align-items: center;
  gap: 0.25em;
  text-overflow: ellipsis;
  min-width: 0;
}

.item-content .item-icon {
  display: contents;
}
</style>
