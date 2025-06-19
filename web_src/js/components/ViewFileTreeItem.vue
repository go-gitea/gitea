<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {isPlainClick} from '../utils/dom.ts';
import {ref} from 'vue';
import {type createViewFileTreeStore} from './ViewFileTreeStore.ts';

type Item = {
  entryName: string;
  entryMode: 'blob' | 'exec' | 'tree' | 'commit' | 'symlink' | 'unknown';
  entryIcon: string;
  entryIconOpen: string;
  fullPath: string;
  submoduleUrl?: string;
  children?: Item[];
};

const props = defineProps<{
  item: Item,
  store: ReturnType<typeof createViewFileTreeStore>
}>();

const store = props.store;
const isLoading = ref(false);
const children = ref(props.item.children);
const collapsed = ref(!props.item.children);

const doLoadChildren = async () => {
  collapsed.value = !collapsed.value;
  if (!collapsed.value) {
    isLoading.value = true;
    try {
      children.value = await store.loadChildren(props.item.fullPath);
    } finally {
      isLoading.value = false;
    }
  }
};

const onItemClick = (e: MouseEvent) => {
  // only handle the click event with page partial reloading if the user didn't press any special key
  // let browsers handle special keys like "Ctrl+Click"
  if (!isPlainClick(e)) return;
  e.preventDefault();
  if (props.item.entryMode === 'tree') doLoadChildren();
  store.navigateTreeView(props.item.fullPath);
};

</script>

<template>
  <a
    class="tree-item silenced"
    :class="{
      'selected': store.selectedItem === item.fullPath,
      'type-submodule': item.entryMode === 'commit',
      'type-directory': item.entryMode === 'tree',
      'type-symlink': item.entryMode === 'symlink',
      'type-file': item.entryMode === 'blob' || item.entryMode === 'exec',
    }"
    :title="item.entryName"
    :href="store.buildTreePathWebUrl(item.fullPath)"
    @click.stop="onItemClick"
  >
    <div v-if="item.entryMode === 'tree'" class="item-toggle">
      <SvgIcon v-if="isLoading" name="octicon-sync" class="circular-spin"/>
      <SvgIcon v-else :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'" @click.stop.prevent="doLoadChildren"/>
    </div>
    <div class="item-content">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span class="tw-contents" v-html="(!collapsed && item.entryIconOpen) ? item.entryIconOpen : item.entryIcon"/>
      <span class="gt-ellipsis">{{ item.entryName }}</span>
    </div>
  </a>

  <div v-if="children?.length" v-show="!collapsed" class="sub-items">
    <ViewFileTreeItem v-for="childItem in children" :key="childItem.entryName" :item="childItem" :store="store"/>
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
  gap: 0.5em;
  text-overflow: ellipsis;
  min-width: 0;
}
</style>
