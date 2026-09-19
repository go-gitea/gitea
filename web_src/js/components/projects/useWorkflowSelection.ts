import {computed, ref} from 'vue';
import {debounce} from '../../utils/func.ts';
import {isPending, isSaved} from './workflowIdentity.ts';
import type {WorkflowEvent, WorkflowStoreState} from './WorkflowStore.ts';

export type WorkflowSelection = ReturnType<typeof useWorkflowSelection>;

/**
 * Owns which workflow is selected and whether it is being edited, including the
 * snapshot that Cancel restores. Selection is always re-resolved through the
 * store by `event_id` rather than by holding a row object, because saving or
 * deleting replaces every row in the list with a freshly fetched one.
 */
export function useWorkflowSelection(store: WorkflowStoreState, options: {
  canWrite: () => boolean;
  onSelected: (row: WorkflowEvent) => void;
}) {
  // Where Cancel returns to. Stored as a key, not a row: a copied row goes stale
  // as soon as the list reloads.
  const previousSelectionKey = ref<string | null>(null);
  const hasSnapshot = ref(false);

  // Tracked here rather than as a flag on the rows themselves, which went stale
  // every time the list was replaced.
  const editModeActive = ref(false);

  // Unsaved workflows are always editable; saved ones follow editModeActive.
  const isInEditMode = computed(() => {
    if (!options.canWrite()) return false;
    if (!store.selectedWorkflow) return false;
    if (store.selectedWorkflow.id === 0) return true;
    return editModeActive.value;
  });

  // Show Cancel only when there is something meaningful to cancel back to.
  const showCancelButton = computed(() => isSaved(store.selectedWorkflow) || isPending(store.selectedWorkflow));

  const resolveByEventId = (eventID: string): WorkflowEvent | null =>
    store.workflowEvents.find((row: WorkflowEvent) => row.event_id === eventID) ?? null;

  const clearSnapshot = () => {
    previousSelectionKey.value = null;
    hasSnapshot.value = false;
  };

  const takeSnapshot = () => {
    previousSelectionKey.value = store.selectedItem;
    hasSnapshot.value = true;
  };

  const selectRow = async (row: WorkflowEvent) => {
    // selectedItem is derived from selectedWorkflow, so matching it here means the
    // row is genuinely loaded rather than merely requested.
    if (store.loading || store.selectedItem === row.event_id) return;
    try {
      editModeActive.value = false; // switching workflows leaves edit mode
      store.selectedWorkflow = row;
      await store.loadWorkflowData(row.event_id);
      options.onSelected(row);
    } catch (error) {
      console.error('Error selecting workflow:', error);
    }
  };

  const selectItem = async (item: WorkflowEvent) => {
    if (store.loading) return;
    // Re-selecting the current row must not clear the snapshot: an in-progress
    // clone keeps its "return to source" anchor there.
    if (store.selectedItem === item.event_id) return;
    clearSnapshot();
    // Match on the key only. A previous workflow_event fallback here resolved two
    // pending clones of the same event type to whichever came first, selecting
    // the wrong row; keys are unique, so the fallback is neither needed nor safe.
    await selectRow(resolveByEventId(item.event_id) ?? item);
  };

  // Sidebar clicks are debounced, so a click can outlive the list it was made
  // against; selectItem re-resolves by key to cope with that.
  const debouncedSelectItem = debounce(async (item: WorkflowEvent) => {
    await selectItem(item);
  }, 150);

  /** Selects by a server-side key, falling back to that event type's placeholder. */
  const selectByEventId = async (eventID: string) => {
    const row = resolveByEventId(eventID) ??
      store.workflowEvents.find((r: WorkflowEvent) => !isSaved(r) && r.workflow_event === eventID);
    if (row) await selectItem(row);
  };

  const selectFirst = async () => {
    const rows = store.workflowEvents;
    if (!rows.length) return;
    await selectItem(rows.find((row: WorkflowEvent) => isSaved(row)) ?? rows[0]);
  };

  const clearSelection = () => {
    store.selectedWorkflow = null;
  };

  const setEditMode = (on: boolean) => {
    editModeActive.value = on;
  };

  const toggleEditMode = async () => {
    if (!options.canWrite()) return;

    if (!isInEditMode.value) {
      takeSnapshot();
      editModeActive.value = true;
      return;
    }

    // Cancelling.
    const canceled = store.selectedWorkflow;
    const wasPending = isPending(canceled);
    if (wasPending && canceled) {
      store.removePendingRow(canceled.event_id);
      store.clearDraft(canceled.event_id);
    } else if (canceled) {
      // Discard unsaved edits so the reload below shows server state rather than
      // the draft the form watchers have been writing.
      store.clearDraft(canceled.event_id);
    }

    const restoreKey = previousSelectionKey.value;
    const restored = hasSnapshot.value && restoreKey ? resolveByEventId(restoreKey) : null;
    if (hasSnapshot.value) {
      if (restored) {
        store.selectedWorkflow = restored;
        await store.loadWorkflowData(restored.event_id);
      } else {
        clearSelection();
      }
      clearSnapshot();
    } else if (wasPending) {
      // No snapshot: fall back to the nearest workflow of the same event type.
      const baseType = canceled?.workflow_event;
      const fallback = store.workflowEvents.find(
        (row: WorkflowEvent) => Boolean(baseType) && (row.workflow_event === baseType || row.event_id === baseType),
      ) ?? store.workflowEvents[0];
      if (fallback) {
        store.selectedWorkflow = fallback;
        await store.loadWorkflowData(fallback.event_id);
      } else {
        clearSelection();
      }
    }
    editModeActive.value = false;
  };

  return {
    isInEditMode,
    showCancelButton,
    resolveByEventId,
    selectRow,
    selectItem,
    debouncedSelectItem,
    selectByEventId,
    selectFirst,
    clearSelection,
    clearSnapshot,
    takeSnapshot,
    setEditMode,
    toggleEditMode,
  };
}
