<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {ref} from 'vue';

type Item = {
  file: any;
  name: string;
  isFile: boolean;
  children?: Item[];
};

const props = defineProps<{
  item: Item,
  selected: string,
  collapsed: boolean,
  fileUrlGetter: any,
  fileClassGetter?: any,
  loadChildren?: any
}>();

const isLoading = ref(false);
const _collapsed = ref(props.collapsed);
const children = ref(props.item.children);

const doLoadChildren = async () => {
  _collapsed.value = !_collapsed.value;
  if (!_collapsed.value && props.loadChildren) {
    isLoading.value = true;
    const _children = await props.loadChildren(props.item);
    children.value = _children;
    isLoading.value = false;
  }
};
</script>

<template>
  <!--title instead of tooltip above as the tooltip needs too much work with the current methods, i.e. not being loaded or staying open for "too long"-->
  <a
    v-if="item.isFile" class="item-file"
    :class="fileClassGetter && fileClassGetter(selected, item)"
    :title="item.name" :href="fileUrlGetter(item)"
  >
    <!-- file -->
    <SvgIcon name="octicon-file"/>
    <span class="gt-ellipsis tw-flex-1">{{ item.name }}</span>
    <slot name="file-item-suffix" :item="item"/>
  </a>
  <div v-else class="item-directory" :title="item.name" @click.stop="doLoadChildren">
    <!-- directory -->
    <SvgIcon v-if="isLoading" name="octicon-sync" class="job-status-rotate"/>
    <SvgIcon v-else :name="_collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'"/>
    <SvgIcon class="text primary" :name="_collapsed ? 'octicon-file-directory-fill' : 'octicon-file-directory-open-fill'"/>
    <span class="gt-ellipsis">{{ item.name }}</span>
  </div>

  <div v-if="children?.length > 0" v-show="!_collapsed" class="sub-items">
    <FileTreeItem
      v-for="childItem in children"
      :key="childItem.name"
      :item="childItem"
      :selected="selected"
      :collapsed="collapsed"
      :file-class-getter="fileClassGetter"
      :file-url-getter="fileUrlGetter"
      :load-children="loadChildren"
    >
      <template #file-item-suffix="{item: treeItem}">
        <slot name="file-item-suffix" :item="treeItem"/>
      </template>
    </FileTreeItem>
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

.item-file.selected {
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
