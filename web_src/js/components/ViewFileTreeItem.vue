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
  getWebUrl:(treePath: string) => string,
  selectedItem?: string,
}>();

const isLoading = ref(false);
const children = ref(props.item.children);
const collapsed = ref(!props.item.children);

const doLoadChildren = async (e?: MouseEvent) => {
  // the event is only not undefined if the user explicitly clicked on the directory item toggle. the preventDefault
  // stops the event from bubbling up and causing a directory content load
  e?.preventDefault();

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

const doLoadDirContent = (e: MouseEvent) => {
  // only load the directory content without a window refresh if the user didn't press any special key
  if (e.button !== 0 || e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return;
  e.preventDefault();

  doLoadChildren();
  props.navigateViewContent(props.item.fullPath);
};

const doLoadFileContent = (e: MouseEvent) => {
  if (e.button !== 0 || e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return;
  e.preventDefault();

  props.navigateViewContent(props.item.fullPath);
};
</script>

<!--title instead of tooltip above as the tooltip needs too much work with the current methods, i.e. not being loaded or staying open for "too long"-->
<template>
  <a
    v-if="item.entryMode === 'commit'" class="tree-item type-submodule"
    :title="item.entryName"
    :href="getWebUrl(item.fullPath)"
  >
    <!-- submodule -->
    <div class="item-content">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span class="tw-contents" v-html="item.entryIcon"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.entryName }}</span>
    </div>
  </a>
  <a
    v-else-if="item.entryMode === 'symlink'" class="tree-item type-symlink"
    :class="{'selected': selectedItem === item.fullPath}"
    :title="item.entryName"
    :href="getWebUrl(item.fullPath)"
    @click.stop="doLoadFileContent"
  >
    <!-- symlink -->
    <div class="item-content">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span class="tw-contents" v-html="item.entryIcon"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.entryName }}</span>
    </div>
  </a>
  <a
    v-else-if="item.entryMode !== 'tree'" class="tree-item type-file"
    :class="{'selected': selectedItem === item.fullPath}"
    :title="item.entryName"
    :href="getWebUrl(item.fullPath)"
    @click.stop="doLoadFileContent"
  >
    <!-- file -->
    <div class="item-content">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span class="tw-contents" v-html="item.entryIcon"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.entryName }}</span>
    </div>
  </a>
  <a
    v-else class="tree-item type-directory"
    :class="{'selected': selectedItem === item.fullPath}"
    :title="item.entryName"
    :href="getWebUrl(item.fullPath)"
    @click.stop="doLoadDirContent"
  >
    <!-- directory -->
    <div class="item-toggle">
      <SvgIcon v-if="isLoading" name="octicon-sync" class="circular-spin"/>
      <SvgIcon v-else :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'" @click.stop="doLoadChildren"/>
    </div>
    <div class="item-content">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <span class="tw-contents" v-html="(!collapsed && item.entryIconOpen) ? item.entryIconOpen : item.entryIcon"/>
      <span class="gt-ellipsis">{{ item.entryName }}</span>
    </div>
  </a>

  <div v-if="children?.length" v-show="!collapsed" class="sub-items">
    <ViewFileTreeItem v-for="childItem in children" :key="childItem.entryName" :item="childItem" :selected-item="selectedItem" :get-web-url="getWebUrl" :navigate-view-content="navigateViewContent" :load-children="loadChildren"/>
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
  color: inherit;
  text-decoration: inherit;
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
