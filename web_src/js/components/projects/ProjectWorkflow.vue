<script lang="ts" setup>
import {onMounted, computed, watch, provide} from 'vue';
import {createWorkflowStore} from './WorkflowStore.ts';
import {buildDisplayNames} from './workflowList.ts';
import {useWorkflowRouting} from './useWorkflowRouting.ts';
import {useWorkflowSelection} from './useWorkflowSelection.ts';
import {useWorkflowClone} from './useWorkflowClone.ts';
import WorkflowSidebar from './WorkflowSidebar.vue';
import WorkflowEditor from './WorkflowEditor.vue';
import type {WorkflowLocale} from './workflowLocale.ts';

const props = defineProps<{
  projectLink: string;
  eventId: string;
  canWriteProjects: boolean;
  locale: WorkflowLocale;
}>();

const store = createWorkflowStore(props);

// Provide the store to WorkflowEditor so it can v-model the form state directly
// without tripping vue/no-mutating-props.
provide('workflowStore', store);

const canWrite = () => props.canWriteProjects;

const routing = useWorkflowRouting(props.projectLink);
const selection = useWorkflowSelection(store, {canWrite, onSelected: routing.pushSelection});
routing.handleNavigation((eventID: string) => void selection.selectByEventId(eventID));

const lifecycle = useWorkflowClone(store, selection, {
  canWrite,
  deleteConfirmText: () => props.locale.deleteConfirm,
  onListEmptied: routing.resetUrl,
});

// Rows are handed to the sidebar untouched. The server already sends
// display_name and is_configured, and copying rows to add derived fields would
// leave the sidebar and the store holding different objects for one workflow.
const displayNames = computed(() => buildDisplayNames(store.workflowEvents));

// Persist the current form state into the draft store whenever it changes.
const persistDraft = () => {
  const key = store.selectedWorkflow?.event_id;
  if (key) store.updateDraft(key, store.workflowFilters, store.workflowActions);
};
watch(() => store.workflowFilters, persistDraft, {deep: true});
watch(() => store.workflowActions, persistDraft, {deep: true});

const saveWorkflow = async () => {
  if (!props.canWriteProjects) return;
  if (!await store.saveWorkflow()) return;
  // Drop a click that is still debouncing, so it cannot override the selection
  // that saving just established.
  selection.debouncedSelectItem.cancel();
  selection.clearSnapshot();
  selection.setEditMode(false);
};

const toggleWorkflowStatus = async () => {
  if (!props.canWriteProjects) return;
  const selected = store.selectedWorkflow;
  if (!selected) return;
  await store.saveWorkflowStatus(!selected.enabled);
};

onMounted(async () => {
  await Promise.all([store.loadEvents(), store.loadProjectOptions()]);

  const deepLink = routing.routableKey(props.eventId);
  if (deepLink) await selection.selectByEventId(deepLink);
  // Either there was no deep link, or it named a workflow that no longer exists.
  if (!store.selectedWorkflow) await selection.selectFirst();
});
</script>

<template>
  <div class="workflow-container">
    <WorkflowSidebar
      :workflows="store.workflowEvents"
      :selected-id="store.selectedItem"
      :heading="locale.defaultWorkflows"
      :display-names="displayNames"
      :href-for="routing.urlFor"
      @select="selection.debouncedSelectItem"
    />
    <WorkflowEditor
      :locale="locale"
      :can-write-projects="canWriteProjects"
      :is-in-edit-mode="selection.isInEditMode.value"
      :show-cancel-button="selection.showCancelButton.value"
      :can-clone-selected-workflow="lifecycle.canClone.value"
      @toggle-edit-mode="selection.toggleEditMode"
      @save-workflow="saveWorkflow"
      @delete-workflow="lifecycle.deleteWorkflow"
      @toggle-workflow-status="toggleWorkflowStatus"
      @clone-workflow="lifecycle.cloneWorkflow"
    />
  </div>
</template>

<style scoped>
.workflow-container {
  display: flex;
  width: 100%;
  height: calc(100vh - 200px);
  min-height: 600px;
  border: 1px solid var(--color-secondary);
  border-radius: 8px;
  overflow: hidden;
  background: var(--color-body);
}
</style>
