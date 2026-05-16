<script lang="ts" setup>
import {onMounted, onUnmounted, computed, ref, watch, provide, toRaw} from 'vue';
import {debounce} from 'throttle-debounce';
import {createWorkflowStore} from './WorkflowStore.ts';
import type {WorkflowEvent} from './WorkflowStore.ts';
import {confirmModal} from '../../features/comp/ConfirmModal.ts';
import WorkflowSidebar from './WorkflowSidebar.vue';
import WorkflowEditor from './WorkflowEditor.vue';

const props = defineProps<{
  projectLink: string;
  eventId: string;
  locale: {
    defaultWorkflows: string;
    moveToColumn: string;
    viewWorkflowConfiguration: string;
    configureWorkflow: string;
    when: string;
    runWhen: string;
    filters: string;
    applyTo: string;
    whenMovedFromColumn: string;
    whenMovedToColumn: string;
    onlyIfHasLabels: string;
    actions: string;
    addLabels: string;
    removeLabels: string;
    anyLabel: string;
    anyColumn: string;
    issueState: string;
    none: string;
    noChange: string;
    edit: string;
    delete: string;
    save: string;
    clone: string;
    cancel: string;
    disable: string;
    disabled: string;
    enabled: string;
    enable: string;
    issuesAndPullRequests: string;
    issuesOnly: string;
    pullRequestsOnly: string;
    selectColumn: string;
    closeIssue: string;
    reopenIssue: string;
    saveWorkflowFailed: string;
    updateWorkflowFailed: string;
    deleteWorkflowFailed: string;
    atLeastOneActionRequired: string;
    cloneTooltip: string;
    deleteConfirm: string;
  };
}>(); 

const store = createWorkflowStore(props);

// Provide store to child components (WorkflowEditor) so they can bind
// v-model directly without triggering vue/no-mutating-props.
provide('workflowStore', store);

// Snapshot stored before entering edit / clone mode so Cancel can restore it.
type SelectionSnapshot = {selectedItem: string | null; selectedWorkflow: WorkflowEvent | null};
const previousSelection = ref<SelectionSnapshot | null>(null);

// ── Edit-mode state ───────────────────────────────────────────────────────────

// Workflows with id=0 are always editable; saved workflows use _isEditing flag.
const isInEditMode = computed(() => {
  if (!store.selectedWorkflow) return false;
  return store.selectedWorkflow.id === 0 || Boolean(store.selectedWorkflow._isEditing);
});

const setEditMode = (on: boolean) => {
  if (store.selectedWorkflow) store.selectedWorkflow._isEditing = on;
};

// Show cancel only when there is something meaningful to cancel to.
const showCancelButton = computed(() => {
  if (!store.selectedWorkflow) return false;
  return store.selectedWorkflow.id > 0 ||
         Boolean(store.selectedWorkflow._clonedFromEventId) ||
         store.selectedWorkflow.event_id.startsWith('clone-');
});

// A workflow is "temporary" (pending unsaved clone) when it has no DB id.
const isTemporaryWorkflow = (wf?: WorkflowEvent | null) => {
  if (!wf || wf.id > 0) return false;
  return Boolean(wf._clonedFromEventId) ||
    wf.event_id.startsWith('clone-') ||
    wf.event_id.startsWith('new-');
};

// Clone is allowed when the selected workflow is saved and has no pending clone.
const canCloneSelectedWorkflow = computed(() => {
  const sel = store.selectedWorkflow;
  if (!sel || sel.id <= 0) return false;
  return !store.workflowEvents.some(
    (w: WorkflowEvent) => w.id === 0 && w._clonedFromEventId === sel.event_id,
  );
});

// ── Sidebar helpers ───────────────────────────────────────────────────────────

// id > 0 means the workflow is saved in the database.
const isWorkflowConfigured = (wf: WorkflowEvent) => wf.id > 0;

const workflowList = computed<WorkflowEvent[]>(() =>
  store.workflowEvents.map((wf: WorkflowEvent) => ({
    ...wf,
    is_configured: isWorkflowConfigured(wf),
    display_name: wf.display_name || wf.workflow_event || wf.event_id,
  }))
);

const getStatusClass = (item: WorkflowEvent) => {
  if (!item.is_configured) return 'status-inactive';
  return item.enabled === false ? 'status-disabled' : 'status-active';
};

const getWorkflowDisplayName = (item: WorkflowEvent, _index: number) => {
  const displayName = item.display_name || item.workflow_event || item.event_id || '';
  const sameType = workflowList.value.filter(
    (w: WorkflowEvent) => w.workflow_event === item.workflow_event && (w.is_configured || w.id === 0),
  );
  if (sameType.length <= 1) return displayName;

  // Sort so saved workflows appear before temporary clones.
  const ordered = [...sameType].sort((a, b) => {
    const at = isTemporaryWorkflow(a), bt = isTemporaryWorkflow(b);
    if (at !== bt) return at ? 1 : -1;
    return workflowList.value.indexOf(a) - workflowList.value.indexOf(b);
  });
  const pos = ordered.findIndex((w: WorkflowEvent) => w.event_id === item.event_id);
  return `${displayName} #${pos + 1}`;
};

// ── Draft persistence ─────────────────────────────────────────────────────────

// Persist the current form state into the draft store whenever filters/actions change.
const persistDraft = () => {
  const key = store.selectedWorkflow?.event_id;
  if (key) store.updateDraft(key, store.workflowFilters, store.workflowActions);
};

watch(() => store.workflowFilters, persistDraft, {deep: true});
watch(() => store.workflowActions, persistDraft, {deep: true});

// ── Navigation ────────────────────────────────────────────────────────────────

const selectWorkflowEvent = async (event: WorkflowEvent) => {
  if (store.loading || store.selectedItem === event.event_id) return;
  try {
    store.selectedItem = event.event_id;
    store.selectedWorkflow = event;
    await store.loadWorkflowData(event.event_id);
    window.history.pushState({event_id: event.event_id}, '', `${props.projectLink}/workflows/${event.event_id}`);
  } catch (error) {
    console.error('Error selecting workflow:', error);
  }
};

const selectWorkflowItem = async (item: WorkflowEvent) => {
  if (store.loading) return;
  // Guard: clicking an already-selected item must never clear previousSelection,
  // because an in-progress clone stores its "return to source" anchor there.
  if (store.selectedItem === item.event_id) return;

  previousSelection.value = null;
  if (item.is_configured) {
    await selectWorkflowEvent(item);
  } else {
    // For unconfigured placeholders, prefer any id=0 object already in the list.
    const existing = store.workflowEvents.find(
      (w: WorkflowEvent) => w.id === 0 && w.workflow_event === item.workflow_event,
    );
    await selectWorkflowEvent(existing || item);
  }
};

const debouncedSelectWorkflowItem = debounce(150, (item: WorkflowEvent) => {
  void selectWorkflowItem(item);
});

// Auto-selects first configured workflow, falling back to first item.
const autoSelectFirstWorkflow = () => {
  const items = workflowList.value;
  if (!items.length) return;
  const first = items.find((i: WorkflowEvent) => i.is_configured) ?? items[0];
  void selectWorkflowItem(first);
};

// Removes a temporary (unsaved) clone from the event list and clears its draft.
const removeTemporaryWorkflow = (wf?: WorkflowEvent | null) => {
  if (!wf || !isTemporaryWorkflow(wf)) return;
  const idx = store.workflowEvents.findIndex((w: WorkflowEvent) => w.event_id === wf.event_id);
  if (idx >= 0) store.workflowEvents.splice(idx, 1);
  store.clearDraft(wf.event_id);
};

// ── Workflow actions ──────────────────────────────────────────────────────────

const toggleEditMode = () => {
  if (!isInEditMode.value) {
    // Enter edit mode: snapshot current selection for Cancel.
    previousSelection.value = {
      selectedItem: store.selectedItem,
      selectedWorkflow: store.selectedWorkflow ? {...store.selectedWorkflow} : null,
    };
    setEditMode(true);
    return;
  }

  // Cancel edit mode.
  const canceled = store.selectedWorkflow;
  const wasTemp = isTemporaryWorkflow(canceled);
  if (wasTemp) removeTemporaryWorkflow(canceled);

  if (previousSelection.value) {
    store.selectedItem = previousSelection.value.selectedItem;
    store.selectedWorkflow = previousSelection.value.selectedWorkflow;
    if (previousSelection.value.selectedWorkflow) {
      void store.loadWorkflowData(previousSelection.value.selectedWorkflow.event_id);
    }
    previousSelection.value = null;
  } else if (wasTemp) {
    // No snapshot: find the nearest workflow of the same event type.
    const baseType = canceled?.workflow_event;
    const fallback =
      store.workflowEvents.find(
        (w: WorkflowEvent) => baseType && (w.workflow_event === baseType || w.event_id === baseType),
      ) ?? store.workflowEvents[0];
    if (fallback) {
      store.selectedItem = fallback.event_id;
      store.selectedWorkflow = fallback;
      void store.loadWorkflowData(fallback.event_id);
    } else {
      store.selectedItem = null;
      store.selectedWorkflow = null;
    }
  }
  setEditMode(false);
};

const saveWorkflow = async () => {
  const ok = await store.saveWorkflow();
  if (ok) {
    previousSelection.value = null;
    setEditMode(false);
  }
};

const toggleWorkflowStatus = async () => {
  if (!store.selectedWorkflow) return;
  store.selectedWorkflow.enabled = !store.selectedWorkflow.enabled;
  await store.saveWorkflowStatus();
};

const deleteWorkflow = async () => {
  const current = store.selectedWorkflow;
  if (!current) return;
  if (!await confirmModal({content: props.locale.deleteConfirm, confirmButtonColor: 'red'})) return;

  if (current.id === 0) {
    // Unsaved temporary workflow: just remove from list.
    const idx = store.workflowEvents.findIndex((w: WorkflowEvent) => w.event_id === current.event_id);
    if (idx >= 0) store.workflowEvents.splice(idx, 1);
  } else {
    await store.deleteWorkflow();
    await store.loadEvents();
  }

  // Select the nearest remaining workflow of the same event type.
  const sameType = store.workflowEvents.filter(
    (w: WorkflowEvent) => w.workflow_event === current.workflow_event,
  );
  let next: WorkflowEvent | null =
    sameType.find((w: WorkflowEvent) => w.is_configured || w.id > 0) ??
    sameType[0] ??
    store.workflowEvents.find((w: WorkflowEvent) => w.is_configured || w.id > 0) ??
    store.workflowEvents[0] ??
    null;

  if (next) {
    await selectWorkflowItem(next);
    if (!next.is_configured && next.id === 0) {
      previousSelection.value = null;
      setEditMode(true);
      return;
    }
  } else {
    store.selectedItem = null;
    store.selectedWorkflow = null;
    window.history.pushState({}, '', `${props.projectLink}/workflows`);
  }
  previousSelection.value = null;
  setEditMode(false);
};

const cloneWorkflow = async (sourceWorkflow?: WorkflowEvent | null) => {
  if (!sourceWorkflow || !canCloneSelectedWorkflow.value) return;

  // Temporary clones use the event-type string as their event_id
  // (e.g. "item_opened") so the backend's "create" path is triggered on save.
  const tempId = sourceWorkflow.workflow_event ?? sourceWorkflow.event_id;
  const cloned: WorkflowEvent = {
    id: 0,
    event_id: tempId,
    display_name: sourceWorkflow.display_name || sourceWorkflow.workflow_event || sourceWorkflow.event_id,
    workflow_event: sourceWorkflow.workflow_event,
    _clonedFromEventId: sourceWorkflow.event_id,
    capabilities: sourceWorkflow.capabilities,
    // toRaw() strips the Vue reactive Proxy before structuredClone; without
    // it the browser throws DataCloneError because Proxies are not cloneable.
    filters: structuredClone(toRaw(sourceWorkflow.filters) ?? []),
    actions: structuredClone(toRaw(sourceWorkflow.actions) ?? []),
    enabled: false,
    is_configured: false,
  };

  // Insert right after the source so same-type workflows stay together.
  const srcIdx = store.workflowEvents.findIndex(
    (w: WorkflowEvent) => w.event_id === sourceWorkflow.event_id,
  );
  if (srcIdx >= 0) store.workflowEvents.splice(srcIdx + 1, 0, cloned);
  else store.workflowEvents.push(cloned);

  // Remember where we came from so Cancel can return.
  previousSelection.value = {
    selectedItem: store.selectedItem,
    selectedWorkflow: store.selectedWorkflow ? {...store.selectedWorkflow} : {...sourceWorkflow},
  };

  store.selectedItem = tempId;
  store.selectedWorkflow = cloned;
  await store.loadWorkflowData(tempId);
  setEditMode(true);

  window.history.pushState({event_id: tempId}, '', `${props.projectLink}/workflows/${tempId}`);
};

// ── Lifecycle ─────────────────────────────────────────────────────────────────

const popstateHandler = (e: PopStateEvent) => {
  if (!e.state?.event_id) return;
  const found = store.workflowEvents.find(
    (ev: WorkflowEvent) => ev.event_id === e.state.event_id,
  );
  if (found) {
    void selectWorkflowEvent(found);
    return;
  }
  // Fallback: unconfigured placeholder for this event type.
  const placeholder = workflowList.value.find(
    (item: WorkflowEvent) =>
      !item.is_configured &&
      (item.workflow_event === e.state.event_id || item.event_id === e.state.event_id),
  );
  if (placeholder) void selectWorkflowEvent(placeholder);
};

onMounted(async () => {
  await Promise.all([store.loadEvents(), store.loadProjectOptions()]);

  if (props.eventId) {
    const exact = store.workflowEvents.find(
      (e: WorkflowEvent) => e.event_id === props.eventId,
    );
    if (exact) {
      store.selectedItem = props.eventId;
      store.selectedWorkflow = exact;
      await store.loadWorkflowData(props.eventId);
    } else {
      const placeholder = workflowList.value.find(
        (item: WorkflowEvent) =>
          !item.is_configured &&
          (item.workflow_event === props.eventId || item.event_id === props.eventId),
      );
      if (placeholder) await selectWorkflowEvent(placeholder);
      else autoSelectFirstWorkflow();
    }
  } else {
    autoSelectFirstWorkflow();
  }

  window.addEventListener('popstate', popstateHandler);
});

onUnmounted(() => {
  debouncedSelectWorkflowItem.cancel();
  window.removeEventListener('popstate', popstateHandler);
});
</script>

<template>
  <div class="workflow-container">
    <WorkflowSidebar
      :workflows="workflowList"
      :selected-id="store.selectedItem"
      :heading="locale.defaultWorkflows"
      :get-display-name="getWorkflowDisplayName"
      :get-status-class="getStatusClass"
      @select="debouncedSelectWorkflowItem"
    />
    <WorkflowEditor
      :locale="locale"
      :is-in-edit-mode="isInEditMode"
      :show-cancel-button="showCancelButton"
      :can-clone-selected-workflow="canCloneSelectedWorkflow"
      @toggle-edit-mode="toggleEditMode"
      @save-workflow="saveWorkflow"
      @delete-workflow="deleteWorkflow"
      @toggle-workflow-status="toggleWorkflowStatus"
      @clone-workflow="cloneWorkflow"
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
