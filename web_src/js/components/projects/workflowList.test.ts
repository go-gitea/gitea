import {buildDisplayNames, mergePendingClones, statusClass} from './workflowList.ts';
import {makePendingCloneKey} from './workflowIdentity.ts';
import type {WorkflowEvent} from './WorkflowStore.ts';

const saved = (id: number, eventType: string, displayName: string, enabled = true): WorkflowEvent =>
  ({id, event_id: String(id), workflow_event: eventType, display_name: displayName, enabled, is_configured: true});

const placeholder = (eventType: string, displayName: string): WorkflowEvent =>
  ({id: 0, event_id: eventType, workflow_event: eventType, display_name: displayName, is_configured: false});

const pendingClone = (source: WorkflowEvent): WorkflowEvent =>
  ({
    id: 0,
    event_id: makePendingCloneKey(source.event_id),
    workflow_event: source.workflow_event,
    display_name: source.display_name,
    _clonedFromEventId: source.event_id,
    is_configured: false,
  });

describe('buildDisplayNames', () => {
  test('leaves a lone workflow unnumbered', () => {
    const rows = [placeholder('item_opened', 'Item opened'), saved(1, 'item_closed', 'Item closed')];
    const names = buildDisplayNames(rows);
    expect(names.get('item_opened')).toBe('Item opened');
    expect(names.get('1')).toBe('Item closed');
  });

  test('numbers workflows that share an event type', () => {
    const rows = [saved(1, 'item_closed', 'Item closed'), saved(2, 'item_closed', 'Item closed')];
    const names = buildDisplayNames(rows);
    expect(names.get('1')).toBe('Item closed #1');
    expect(names.get('2')).toBe('Item closed #2');
  });

  test('numbers pending clones after the saved workflows they came from', () => {
    const first = saved(1, 'item_closed', 'Item closed');
    const second = saved(2, 'item_closed', 'Item closed');
    const cloneOfFirst = pendingClone(first);
    const cloneOfSecond = pendingClone(second);
    // Interleaved exactly as the sidebar shows them, to prove ordering is by
    // saved-before-pending rather than by position in the list.
    const names = buildDisplayNames([first, cloneOfFirst, second, cloneOfSecond]);

    expect(names.get('1')).toBe('Item closed #1');
    expect(names.get('2')).toBe('Item closed #2');
    expect(names.get(cloneOfFirst.event_id)).toBe('Item closed #3');
    expect(names.get(cloneOfSecond.event_id)).toBe('Item closed #4');
  });

  test('two pending clones of the same event type get distinct names', () => {
    const source = saved(1, 'item_closed', 'Item closed');
    const cloneA = pendingClone(source);
    const cloneB = pendingClone(source);
    const names = buildDisplayNames([source, cloneA, cloneB]);
    expect(cloneA.event_id).not.toBe(cloneB.event_id);
    expect(names.get(cloneA.event_id)).not.toBe(names.get(cloneB.event_id));
  });

  test('does not number across different event types', () => {
    const names = buildDisplayNames([saved(1, 'item_opened', 'Item opened'), saved(2, 'item_closed', 'Item closed')]);
    expect(names.get('1')).toBe('Item opened');
    expect(names.get('2')).toBe('Item closed');
  });
});

describe('statusClass', () => {
  test('an unconfigured placeholder is inactive', () => {
    expect(statusClass(placeholder('item_opened', 'Item opened'))).toBe('status-inactive');
  });

  test('a pending clone is inactive until it is saved', () => {
    expect(statusClass(pendingClone(saved(1, 'item_closed', 'Item closed')))).toBe('status-inactive');
  });

  test('a saved workflow reflects its enabled flag', () => {
    expect(statusClass(saved(1, 'item_closed', 'Item closed', true))).toBe('status-active');
    expect(statusClass(saved(1, 'item_closed', 'Item closed', false))).toBe('status-disabled');
  });
});

describe('mergePendingClones', () => {
  test('returns the server list untouched when nothing is pending', () => {
    const serverRows = [saved(1, 'item_opened', 'Item opened')];
    expect(mergePendingClones(serverRows, [])).toBe(serverRows);
  });

  test('re-attaches a pending clone directly after its source', () => {
    const source = saved(1, 'item_closed', 'Item closed');
    const clone = pendingClone(source);
    const merged = mergePendingClones([saved(1, 'item_closed', 'Item closed'), saved(9, 'item_opened', 'Item opened')], [clone]);
    expect(merged.map((row) => row.event_id)).toEqual(['1', clone.event_id, '9']);
  });

  test('keeps both pending clones of the same event type across a reload', () => {
    // The regression: saving one clone reloaded the list and silently dropped
    // the other, along with leaking its draft.
    const sourceA = saved(1, 'item_closed', 'Item closed');
    const sourceB = saved(2, 'item_closed', 'Item closed');
    const cloneA = pendingClone(sourceA);
    const cloneB = pendingClone(sourceB);

    const merged = mergePendingClones(
      [saved(1, 'item_closed', 'Item closed'), saved(2, 'item_closed', 'Item closed')],
      [cloneA, cloneB],
    );

    expect(merged.map((row) => row.event_id)).toEqual(['1', cloneA.event_id, '2', cloneB.event_id]);
  });

  test('falls back to the end of the event-type group when the source was deleted', () => {
    const source = saved(1, 'item_closed', 'Item closed');
    const clone = pendingClone(source);
    const merged = mergePendingClones([saved(2, 'item_closed', 'Item closed'), saved(9, 'item_opened', 'Item opened')], [clone]);
    expect(merged.map((row) => row.event_id)).toEqual(['2', clone.event_id, '9']);
  });

  test('appends when no workflow of that event type remains', () => {
    const clone = pendingClone(saved(1, 'item_closed', 'Item closed'));
    const merged = mergePendingClones([saved(9, 'item_opened', 'Item opened')], [clone]);
    expect(merged.map((row) => row.event_id)).toEqual(['9', clone.event_id]);
  });
});
