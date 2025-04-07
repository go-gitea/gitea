<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {ref} from 'vue';

type Item = {
  entryName: string;
  entryMode: string;
  entryIcon: string;
  entryIconOpen: string;
  fullPath: string;
  submoduleUrl?: string;
  children?: Item[];
};

const props = defineProps<{
  item: Item,
  navigateViewContent:(treePath: string) => void,
  loadChildren:(treePath: string, subPath?: string) => Promise<Item[]>,
  selectedItem?: string,
}>();

const isLoading = ref(false);
const children = ref(props.item.children);
const collapsed = ref(!props.item.children);

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
</script>

<!--title instead of tooltip above as the tooltip needs too much work with the current methods, i.e. not being loaded or staying open for "too long"-->
<template>
  <div
    v-if="item.entryMode === 'commit'" class="tree-item type-submodule"
    :title="item.entryName"
    @click.stop="doGotoSubModule"
  >
    <!-- submodule -->
    <div class="item-content">
      <SvgIcon class="text primary" name="octicon-file-submodule"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.entryName }}</span>
    </div>
  </div>
  <div
    v-else-if="item.entryMode === 'symlink'" class="tree-item type-symlink"
    :class="{'selected': selectedItem === item.fullPath}"
    :title="item.entryName"
    @click.stop="doLoadFileContent"
  >
    <!-- symlink -->
    <div class="item-content">
      <SvgIcon name="octicon-file-symlink-file"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.entryName }}</span>
    </div>
  </div>
  <div
    v-else-if="item.entryMode !== 'tree'" class="tree-item type-file"
    :class="{'selected': selectedItem === item.fullPath}"
    :title="item.entryName"
    @click.stop="doLoadFileContent"
  >
    <!-- file -->
    <div class="item-content">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span v-html="item.entryIcon"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.entryName }}</span>
    </div>
  </div>
  <div
    v-else class="tree-item type-directory"
    :class="{'selected': selectedItem === item.fullPath}"
    :title="item.entryName"
    @click.stop="doLoadDirContent"
  >
    <!-- directory -->
    <div class="item-toggle">
      <!-- FIXME: use a general and global class for this animation -->
      <SvgIcon v-if="isLoading" name="octicon-sync" class="job-status-rotate"/>
      <SvgIcon v-else :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'" @click.stop="doLoadChildren"/>
    </div>
    <div class="item-content">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span class="text primary" v-html="(!collapsed && item.entryIconOpen) ? item.entryIconOpen : item.entryIcon"/>
      <span class="gt-ellipsis">{{ item.entryName }}</span>
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
</style>
