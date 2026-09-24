import {createWorkflowStore} from './WorkflowStore.ts';
import {useWorkflowSelection} from './useWorkflowSelection.ts';
import type {WorkflowEvent, WorkflowStoreState} from './WorkflowStore.ts';

const locale = {
  atLeastOneActionRequired: 'at least one action',
  saveWorkflowFailed: 'save failed',
  updateWorkflowFailed: 'update failed',
  deleteWorkflowFailed: 'delete failed',
  unexpectedResponseFormat: 'unexpected response',
  unexpectedError: 'unexpected error',
};

const saved = (id: number, eventType: string): WorkflowEvent =>
  ({id, event_id: String(id), workflow_event: eventType, display_name: eventType, enabled: true, is_configured: true});

const placeholder = (eventType: string): WorkflowEvent =>
  ({id: 0, event_id: eventType, workflow_event: eventType, display_name: eventType, is_configured: false});

function setup(rows: WorkflowEvent[]) {
  const store: WorkflowStoreState = createWorkflowStore({projectLink: '/owner/repo/projects/1', locale});
  store.workflowEvents = rows;
  const selected: WorkflowEvent[] = [];
  const selection = useWorkflowSelection(store, {
    canWrite: () => true,
    onSelected: (row) => {
      selected.push(row);
    },
  });
  return {store, selection, selected};
}

describe('selection invariant', () => {
  test('a fresh store reports no selection at all', () => {
    // selectedItem used to be seeded from the deep link, so it named a workflow
    // that had never been loaded and selection then refused to load it.
    const {store} = setup([saved(42, 'item_opened')]);
    expect(store.selectedWorkflow).toBe(null);
    expect(store.selectedItem).toBe(null);
  });

  test('selectedItem always agrees with the loaded workflow', async () => {
    const {store, selection} = setup([saved(42, 'item_opened'), saved(43, 'item_closed')]);

    await selection.selectByEventId('42');
    expect(store.selectedItem).toBe(store.selectedWorkflow!.event_id);
    expect(store.selectedItem).toBe('42');

    await selection.selectByEventId('43');
    expect(store.selectedItem).toBe(store.selectedWorkflow!.event_id);
    expect(store.selectedItem).toBe('43');

    selection.clearSelection();
    expect(store.selectedWorkflow).toBe(null);
    expect(store.selectedItem).toBe(null);
  });
});

describe('deep linking', () => {
  test('deep-linking a saved workflow loads it into the editor pane', async () => {
    const {store, selection} = setup([placeholder('item_closed'), saved(42, 'item_opened')]);

    await selection.selectByEventId('42');

    // The regression showed up as a correctly highlighted sidebar next to an
    // empty pane, i.e. selectedItem set while selectedWorkflow stayed null.
    expect(store.selectedWorkflow).not.toBe(null);
    expect(store.selectedWorkflow!.id).toBe(42);
    expect(store.selectedItem).toBe('42');
  });

  test('deep-linking the only saved workflow still loads it', async () => {
    // With one saved row, the selectFirst() fallback targets that same row, so a
    // key-equality early return skipped both attempts and left the pane empty.
    const {store, selection} = setup([saved(42, 'item_opened'), placeholder('item_closed')]);

    await selection.selectByEventId('42');
    if (!store.selectedWorkflow) await selection.selectFirst();

    expect(store.selectedWorkflow).not.toBe(null);
    expect(store.selectedWorkflow!.id).toBe(42);
  });

  test('deep-linking an unconfigured event type selects its placeholder', async () => {
    const {store, selection} = setup([saved(42, 'item_opened'), placeholder('item_closed')]);

    await selection.selectByEventId('item_closed');

    expect(store.selectedWorkflow!.event_id).toBe('item_closed');
    expect(store.selectedItem).toBe('item_closed');
  });

  test('a deep link naming nothing leaves the selection empty for the caller to fall back', async () => {
    const {store, selection} = setup([saved(42, 'item_opened')]);

    await selection.selectByEventId('999');
    expect(store.selectedWorkflow).toBe(null);

    await selection.selectFirst();
    expect(store.selectedWorkflow!.id).toBe(42);
  });
});

describe('re-selecting the current row', () => {
  test('is a no-op, so an in-progress clone keeps its anchor', async () => {
    const {store, selection, selected} = setup([saved(42, 'item_opened'), saved(43, 'item_opened')]);

    await selection.selectByEventId('42');
    expect(selected).toHaveLength(1);

    // Guarding on key equality is still correct now that the key is derived: it
    // can only match once the row is really loaded.
    await selection.selectItem(store.workflowEvents[0]);
    expect(selected).toHaveLength(1);
    expect(store.selectedWorkflow!.id).toBe(42);
  });
});
