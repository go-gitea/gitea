import {
  isPending,
  isPendingKey,
  isSaved,
  makePendingCloneKey,
  saveTargetId,
  urlKeyFor,
} from './workflowIdentity.ts';
import type {WorkflowEvent} from './WorkflowStore.ts';

const savedWorkflow = (id: number, eventType = 'item_opened'): WorkflowEvent =>
  ({id, event_id: String(id), workflow_event: eventType});

const placeholder = (eventType = 'item_opened'): WorkflowEvent =>
  ({id: 0, event_id: eventType, workflow_event: eventType});

const pendingClone = (source: WorkflowEvent): WorkflowEvent =>
  ({id: 0, event_id: makePendingCloneKey(source.event_id), workflow_event: source.workflow_event, _clonedFromEventId: source.event_id});

describe('makePendingCloneKey', () => {
  test('mints a distinct key for every clone of the same source', () => {
    const source = savedWorkflow(5);
    const keys = [
      makePendingCloneKey(source.event_id),
      makePendingCloneKey(source.event_id),
      makePendingCloneKey(source.event_id),
    ];
    expect(new Set(keys).size).toBe(3);
    for (const key of keys) expect(isPendingKey(key)).toBe(true);
  });

  test('two saved workflows of the same event type produce non-colliding keys', () => {
    // The original bug: both clones shared an id, so selecting one selected the other.
    const first = makePendingCloneKey(savedWorkflow(5, 'item_closed').event_id);
    const second = makePendingCloneKey(savedWorkflow(6, 'item_closed').event_id);
    expect(first).not.toBe(second);
  });
});

describe('isPendingKey', () => {
  test('rejects keys the server produces', () => {
    expect(isPendingKey('42')).toBe(false);
    expect(isPendingKey('item_opened')).toBe(false);
  });
});

describe('isPending / isSaved', () => {
  test('classifies each kind of row', () => {
    const saved = savedWorkflow(42);
    const unsaved = placeholder();
    const clone = pendingClone(saved);

    expect([isSaved(saved), isPending(saved)]).toEqual([true, false]);
    expect([isSaved(unsaved), isPending(unsaved)]).toEqual([false, false]);
    expect([isSaved(clone), isPending(clone)]).toEqual([false, true]);
  });

  test('treats a missing row as neither', () => {
    expect(isSaved(null)).toBe(false);
    expect(isPending(undefined)).toBe(false);
  });
});

describe('urlKeyFor', () => {
  test('a saved workflow addresses itself by database id', () => {
    expect(urlKeyFor(savedWorkflow(42))).toBe('42');
  });

  test('a placeholder addresses itself by event type', () => {
    expect(urlKeyFor(placeholder('item_closed'))).toBe('item_closed');
  });

  test('a pending clone reports its source, never its own client key', () => {
    // Routing the client key would 404 the reload and silently select something else.
    const clone = pendingClone(savedWorkflow(7));
    expect(urlKeyFor(clone)).toBe('7');
    expect(isPendingKey(urlKeyFor(clone)!)).toBe(false);
  });

  test('is null when there is nothing addressable', () => {
    expect(urlKeyFor(null)).toBe(null);
    expect(urlKeyFor({id: 0, event_id: makePendingCloneKey('9'), workflow_event: 'item_opened'})).toBe(null);
  });
});

describe('saveTargetId', () => {
  test('an existing workflow is updated by database id', () => {
    expect(saveTargetId(savedWorkflow(42))).toBe('42');
  });

  test('an unsaved row is created by event type', () => {
    expect(saveTargetId(placeholder('item_reopened'))).toBe('item_reopened');
    expect(saveTargetId(pendingClone(savedWorkflow(5, 'item_closed')))).toBe('item_closed');
  });
});
