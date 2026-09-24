import {computed, toRaw} from 'vue';
import {confirmModal} from '../../features/comp/ConfirmModal.ts';
import {isPending, isSaved, makePendingCloneKey} from './workflowIdentity.ts';
import type {WorkflowSelection} from './useWorkflowSelection.ts';
import type {WorkflowEvent, WorkflowStoreState} from './WorkflowStore.ts';

/**
 * Owns adding and removing rows: cloning a saved workflow into an unsaved copy,
 * and deleting a workflow along with choosing what to select afterwards.
 */
export function useWorkflowClone(store: WorkflowStoreState, selection: WorkflowSelection, options: {
  canWrite: () => boolean;
  deleteConfirmText: () => string;
  onListEmptied: () => void;
}) {
  // Cloning is offered for a saved workflow that has no pending clone already.
  const canClone = computed(() => {
    if (!options.canWrite()) return false;
    const selected = store.selectedWorkflow;
    if (!isSaved(selected)) return false;
    return store.workflowEvents.every(
      (row: WorkflowEvent) => !isPending(row) || row._clonedFromEventId !== selected!.event_id,
    );
  });

  const cloneWorkflow = async (source?: WorkflowEvent | null) => {
    if (!options.canWrite() || !source || !canClone.value) return;

    const key = makePendingCloneKey(source.event_id);
    const cloned: WorkflowEvent = {
      id: 0,
      event_id: key,
      display_name: source.display_name || source.workflow_event || source.event_id,
      workflow_event: source.workflow_event,
      _clonedFromEventId: source.event_id,
      capabilities: source.capabilities,
      // toRaw() strips the Vue reactive Proxy before structuredClone, which
      // cannot clone Proxies and would otherwise throw DataCloneError.
      filters: structuredClone(toRaw(source.filters) ?? []),
      actions: structuredClone(toRaw(source.actions) ?? []),
      enabled: false,
      is_configured: false,
    };

    // Insert right after the source so same-type workflows stay together.
    const at = store.workflowEvents.findIndex((row: WorkflowEvent) => row.event_id === source.event_id);
    if (at >= 0) store.workflowEvents.splice(at + 1, 0, cloned);
    else store.workflowEvents.push(cloned);

    // Anchor Cancel to the source before the selection moves off it. The URL is
    // deliberately left alone: a pending clone is not addressable.
    selection.takeSnapshot();
    store.selectedWorkflow = cloned;
    await store.loadWorkflowData(key);
    selection.setEditMode(true);
  };

  const deleteWorkflow = async () => {
    if (!options.canWrite()) return;
    const current = store.selectedWorkflow;
    if (!current) return;
    if (!await confirmModal({content: options.deleteConfirmText(), confirmButtonColor: 'red'})) return;

    if (current.id === 0) {
      store.removePendingRow(current.event_id);
      store.clearDraft(current.event_id);
    } else {
      await store.deleteWorkflow();
      await store.loadEvents();
    }

    // Prefer the nearest remaining workflow of the same event type.
    const sameType = store.workflowEvents.filter(
      (row: WorkflowEvent) => row.workflow_event === current.workflow_event,
    );
    const next: WorkflowEvent | null =
      sameType.find((row: WorkflowEvent) => isSaved(row)) ??
      sameType[0] ??
      store.workflowEvents.find((row: WorkflowEvent) => isSaved(row)) ??
      store.workflowEvents[0] ??
      null;

    selection.clearSnapshot();
    if (!next) {
      selection.clearSelection();
      options.onListEmptied();
      selection.setEditMode(false);
      return;
    }

    await selection.selectItem(next);
    // An unconfigured placeholder opens straight into edit mode.
    selection.setEditMode(options.canWrite() && !isSaved(next));
  };

  return {canClone, cloneWorkflow, deleteWorkflow};
}
