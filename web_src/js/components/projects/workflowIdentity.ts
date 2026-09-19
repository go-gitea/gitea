import type {WorkflowEvent} from './WorkflowStore.ts';

// `event_id` means three different things depending on which row it came from:
//   * a saved workflow            -> its database id as a string, e.g. "42"
//   * an unconfigured placeholder -> the event-type string, e.g. "item_opened"
//   * a pending clone             -> a browser-only key minted here, e.g. "clone-42-1"
// Every "which kind of row is this?" test lives in this module rather than being
// open-coded at each call site: the two selection bugs fixed earlier both came
// from two call sites disagreeing about what the field meant.

const pendingClonePrefix = 'clone-';

let pendingCloneSeq = 0;

/**
 * Mints the browser-only key for a pending clone. The key must be unique across
 * all rows, because selection, draft keying and removal all match on it, and
 * cloning two saved workflows of the same event type would otherwise collide.
 */
export function makePendingCloneKey(sourceEventID: string): string {
  pendingCloneSeq += 1;
  return `${pendingClonePrefix}${sourceEventID}-${pendingCloneSeq}`;
}

/** True when the key was minted by this module rather than sent by the server. */
export function isPendingKey(eventID: string): boolean {
  return eventID.startsWith(pendingClonePrefix);
}

/** True for a clone that exists only in the browser and has never been saved. */
export function isPending(wf?: WorkflowEvent | null): boolean {
  if (!wf || wf.id > 0) return false;
  return Boolean(wf._clonedFromEventId) || isPendingKey(wf.event_id);
}

/** True for a workflow that exists in the database. */
export function isSaved(wf?: WorkflowEvent | null): boolean {
  return Boolean(wf && wf.id > 0);
}

/**
 * The key the server understands for this row, or null when there is none.
 * A pending clone has no server-side address, so it reports the workflow it was
 * cloned from: that is the page a reload should land on.
 */
export function urlKeyFor(wf?: WorkflowEvent | null): string | null {
  if (!wf) return null;
  if (!isPending(wf)) return wf.event_id;
  return wf._clonedFromEventId ?? null;
}

/**
 * The value to POST as `event_id`. The backend parses it as a database id to
 * update an existing workflow and otherwise as an event type to create a new
 * one, so an unsaved row must send its event type instead of its client key.
 */
export function saveTargetId(wf: WorkflowEvent): string {
  return wf.id === 0 ? wf.workflow_event! : wf.event_id;
}
