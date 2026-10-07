<script lang="ts" setup>
import {onMounted, onUnmounted} from 'vue';
import SvgIcon from './SvgIcon.vue';
import ProjectWorkflowEditor from './ProjectWorkflowEditor.vue';
import {isPlainClick} from '../utils/dom.ts';
import {createProjectWorkflowStore, type ProjectWorkflowLocale, type WorkflowRow} from './ProjectWorkflowStore.ts';

const props = defineProps<{
  projectLink: string;
  workflowKey: string;
  canWrite: boolean;
  locale: ProjectWorkflowLocale;
}>();

const store = createProjectWorkflowStore(props);

const statusClass = (row: WorkflowRow) => {
  if (!row.workflow) return 'tw-text-text-light-2';
  return row.workflow.enabled ? 'tw-text-green' : 'tw-text-red';
};

const onRowClick = (e: MouseEvent, row: WorkflowRow) => {
  if (!isPlainClick(e)) return;
  e.preventDefault();
  if (row.key !== store.selectedKey) store.select(row, 'push');
};

const onPopState = (e: PopStateEvent) => {
  if (!e.state?.key) return;
  const row = store.resolve(e.state.key) ?? store.resolve(e.state.event);
  if (row) store.select(row);
};

onMounted(async () => {
  window.addEventListener('popstate', onPopState);
  await store.load();
  const row = store.resolve(props.workflowKey) ?? store.rows.find((r) => r.workflow) ?? store.rows[0];
  if (row) store.select(row, 'replace');
});

onUnmounted(() => window.removeEventListener('popstate', onPopState));
</script>

<template>
  <div class="flex-container">
    <div class="flex-container-nav">
      <nav class="ui fluid vertical menu" :aria-label="locale.defaultWorkflows">
        <div class="header item">{{ locale.defaultWorkflows }}</div>
        <a
          v-for="row in store.rows"
          :key="row.key"
          class="item flex-text-block"
          :class="{active: row.key === store.selectedKey}"
          :href="store.urlFor(row)"
          :aria-current="row.key === store.selectedKey ? 'page' : undefined"
          @click="onRowClick($event, row)"
        >
          <SvgIcon
            name="octicon-dot-fill" :class="statusClass(row)" role="img"
            :aria-label="row.workflow ? (row.workflow.enabled ? locale.enabled : locale.disabled) : undefined"
            :aria-hidden="row.workflow ? undefined : 'true'"
          />
          <div class="tw-min-w-0">
            <div class="gt-ellipsis">{{ row.displayName }}</div>
            <div v-if="store.summary(row)" class="gt-ellipsis tw-text-12 tw-text-text-light-2">{{ store.summary(row) }}</div>
          </div>
        </a>
      </nav>
    </div>
    <ProjectWorkflowEditor v-if="store.selected" class="flex-container-main" :store="store" :locale="locale"/>
  </div>
</template>
