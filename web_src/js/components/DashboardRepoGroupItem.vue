<script lang="ts" setup>
import {computed, inject, type WritableComputedRef} from 'vue';
import {commitStatus, type CommitStatus, type GroupMapType} from './DashboardRepoList.vue';
import DashboardRepoGroup from './DashboardRepoGroup.vue';
import {SvgIcon, type SvgName} from '../svg.ts';
const {depth} = defineProps<{index: number; depth: number;}>();
const groupData = inject<WritableComputedRef<Map<number, GroupMapType>>>('groups');
const loadedMap = inject<WritableComputedRef<Map<number, boolean>>>('loadedMap');
const expandedGroups = inject<WritableComputedRef<number[]>>('expandedGroups');
const itemProp = defineModel<any>('item');
const isGroup = computed<boolean>(() => typeof itemProp.value === 'number');
const item = computed(() => isGroup.value ? groupData.value.get(itemProp.value as number) : itemProp.value);
const id = computed(() => typeof itemProp.value === 'number' ? itemProp.value : itemProp.value.id);
const idKey = computed<string>(() => {
  const prefix = isGroup.value ? 'group' : 'repo';
  return `${prefix}-${id.value}`;
});

const indentCss = computed<string>(() => `padding-inline-start: ${depth * 0.5}rem`);

function icon(item: any) {
  if (item.repos) {
    return 'octicon-list-unordered';
  }
  if (item.fork) {
    return 'octicon-repo-forked';
  } else if (item.mirror) {
    return 'octicon-mirror';
  } else if (item.template) {
    return `octicon-repo-template`;
  } else if (item.private) {
    return 'octicon-lock';
  } else if (item.internal) {
    return 'octicon-repo';
  }
  return 'octicon-repo';
}

function statusIcon(status: CommitStatus): SvgName {
  return commitStatus[status].name as SvgName;
}

function statusColor(status: CommitStatus) {
  return commitStatus[status].color;
}
const emitter = defineEmits<{
  loadRequested: [ number ]
}>();
function onCheck(nv: boolean) {
  if (isGroup.value && expandedGroups) {
    if (nv) {
      expandedGroups.value = [...expandedGroups.value, item.value.id];
      if (!loadedMap.value.has(item.value.id)) {
        emitter('loadRequested', item.value.id as number);
        loadedMap.value.set(item.value.id, true);
      }
    } else {
      const idx = expandedGroups.value.indexOf(item.value.id as number);
      if (idx > -1) {
        expandedGroups.value = expandedGroups.value.toSpliced(idx, 1);
      }
    }
  }
}
const active = computed(() => isGroup.value && expandedGroups.value.includes(id.value));
</script>
<template>
  <li class="tw-flex tw-flex-col tw-px-0 tw-pr-0 expandable-menu-item tw-mt-0" :data-sort-id="idKey" :data-is-group="isGroup" :data-id="id">
    <label
      class="tw-flex tw-content-center tw-py-2 tw-space-x-1.5"
      :style="indentCss"
      :class="{
        'has-children': !!item.repos?.length || !!item.subgroups?.length || isGroup,
      }"
    >
      <input v-if="isGroup" :checked="active" type="checkbox" class="toggle tw-h-0 tw-w-0 tw-overflow-hidden tw-opacity-0 tw-absolute" @change="(e) => onCheck((e.target as HTMLInputElement).checked)">
      <svg-icon :name="icon(item)" :size="16" class-name="repo-list-icon"/>
      <svg-icon v-if="isGroup" name="octicon-chevron-right" :size="16" class="collapse-icon"/>
      <a :href="item.link" class="repo-list-link muted tw-flex-shrink">
        <div class="text truncate tw-flex-shrink">{{ item.full_name || item.name }}</div>
        <div v-if="item.archived">
          <svg-icon name="octicon-archive" :size="16"/>
        </div>
      </a>
      <a class="tw-flex tw-items-center" v-if="item.latest_commit_status_state" :href="item.latest_commit_status_state_link" :data-tooltip-content="item.locale_latest_commit_status_state">
        <!-- the commit status icon logic is taken from templates/repo/commit_status.tmpl -->
        <svg-icon :name="statusIcon(item.latest_commit_status_state)" :class-name="'commit-status icon text ' + statusColor(item.latest_commit_status_state)" :size="16"/>
      </a>
    </label>
    <div class="menu-expandable-content">
      <div class="menu-expandable-content-inner">
        <dashboard-repo-group :cur-group="id" v-if="isGroup" :depth="depth + 1"/>
      </div>
    </div>
  </li>
</template>
<style scoped>
.repo-list-link {
  min-width: 0;
  /* for text truncation */
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
</style>
