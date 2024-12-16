<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {ref} from 'vue';

type Item = {
  name: string;
  path: string;
  htmlUrl: string;
  isFile: boolean;
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
    const _children = await props.loadChildren(props.item);
    children.value = _children;
    isLoading.value = false;
  }
};

const doLoadDirContent = () => {
  doLoadChildren();
  props.loadContent(props.item);
};

const doLoadFileContent = () => {
  props.loadContent(props.item);
};
</script>

<template>
  <!--title instead of tooltip above as the tooltip needs too much work with the current methods, i.e. not being loaded or staying open for "too long"-->
  <div
    v-if="item.isFile" class="item-file"
    :class="{'selected': selectedItem.value === item.path}"
    :title="item.name"
    @click.stop="doLoadFileContent"
  >
    <!-- file -->
    <SvgIcon name="octicon-file"/>
    <span class="gt-ellipsis tw-flex-1">{{ item.name }}</span>
  </div>
  <div
    v-else class="item-directory"
    :class="{'selected': selectedItem.value === item.path}"
    :title="item.name"
    @click.stop="doLoadDirContent"
  >
    <!-- directory -->
    <SvgIcon v-if="isLoading" name="octicon-sync" class="job-status-rotate"/>
    <SvgIcon v-else :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'" @click.stop="doLoadChildren"/>
    <SvgIcon class="text primary" :name="collapsed ? 'octicon-file-directory-fill' : 'octicon-file-directory-open-fill'"/>
    <span class="gt-ellipsis">{{ item.name }}</span>
  </div>

  <div v-if="children?.length" v-show="!collapsed" class="sub-items">
    <ViewFileTreeItem v-for="childItem in children" :key="childItem.name" :item="childItem" :selected-item="selectedItem" :load-content="loadContent" :load-children="loadChildren"/>
  </div>
</template>
<style scoped>
a, a:hover {
  text-decoration: none;
  color: var(--color-text);
}

.sub-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-left: 13px;
  border-left: 1px solid var(--color-secondary);
}

.sub-items .item-file {
  padding-left: 18px;
}

.item-directory.selected, .item-file.selected {
  color: var(--color-text);
  background: var(--color-active);
  border-radius: 4px;
}

.item-file.viewed {
  color: var(--color-text-light-3);
}

.item-directory {
  user-select: none;
}

.item-file,
.item-directory {
  display: flex;
  align-items: center;
  gap: 0.25em;
  padding: 6px;
}

.item-file:hover,
.item-directory:hover {
  color: var(--color-text);
  background: var(--color-hover);
  border-radius: 4px;
  cursor: pointer;
}
</style>
