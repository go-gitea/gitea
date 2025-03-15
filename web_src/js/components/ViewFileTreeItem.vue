<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {ref} from 'vue';

type Item = {
  name: string;
  path: string;
  sub_module_url?: string;
  type: string;
  children?: Item[];
};

const props = defineProps<{
  item: Item,
  loadContent: any;
  loadChildren: any;
  selectedItem?: any;
}>();

const isLoading = ref(false);
const children = ref(props.item.children);
const collapsed = ref(!props.item.children);

const doLoadChildren = async () => {
  collapsed.value = !collapsed.value;
  if (!collapsed.value && props.loadChildren) {
    isLoading.value = true;
    try {
      children.value = await props.loadChildren(props.item.path);
    } finally {
      isLoading.value = false;
    }
  }
};

const doLoadDirContent = () => {
  doLoadChildren();
  props.loadContent(props.item.path);
};

const doLoadFileContent = () => {
  props.loadContent(props.item.path);
};

const doGotoSubModule = () => {
  location.href = props.item.sub_module_url;
};
</script>

<template>
  <!--title instead of tooltip above as the tooltip needs too much work with the current methods, i.e. not being loaded or staying open for "too long"-->
  <div
    v-if="item.type === 'commit'" class="tree-item type-submodule"
    :title="item.name"
    @click.stop="doGotoSubModule"
  >
    <!-- submodule -->
    <div class="item-content">
      <SvgIcon class="text primary" name="octicon-file-submodule"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.name }}</span>
    </div>
  </div>
  <div
    v-else-if="item.type === 'symlink'" class="tree-item type-symlink"
    :class="{'selected': selectedItem.value === item.path}"
    :title="item.name"
    @click.stop="doLoadFileContent"
  >
    <!-- symlink -->
    <div class="item-content">
      <SvgIcon name="octicon-file-symlink-file"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.name }}</span>
    </div>
  </div>
  <div
    v-else-if="item.type !== 'tree'" class="tree-item type-file"
    :class="{'selected': selectedItem.value === item.path}"
    :title="item.name"
    @click.stop="doLoadFileContent"
  >
    <!-- file -->
    <div class="item-content">
      <SvgIcon name="octicon-file"/>
      <span class="gt-ellipsis tw-flex-1">{{ item.name }}</span>
    </div>
  </div>
  <div
    v-else class="tree-item type-directory"
    :class="{'selected': selectedItem.value === item.path}"
    :title="item.name"
    @click.stop="doLoadDirContent"
  >
    <!-- directory -->
    <div class="item-toggle">
      <SvgIcon v-if="isLoading" name="octicon-sync" class="job-status-rotate"/>
      <SvgIcon v-else :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'" @click.stop="doLoadChildren"/>
    </div>
    <div class="item-content">
      <SvgIcon class="text primary" :name="collapsed ? 'octicon-file-directory-fill' : 'octicon-file-directory-open-fill'"/>
      <span class="gt-ellipsis">{{ item.name }}</span>
    </div>
  </div>

  <div v-if="children?.length" v-show="!collapsed" class="sub-items">
    <ViewFileTreeItem v-for="childItem in children" :key="childItem.name" :item="childItem" :selected-item="selectedItem" :load-content="loadContent" :load-children="loadChildren"/>
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
}
</style>
