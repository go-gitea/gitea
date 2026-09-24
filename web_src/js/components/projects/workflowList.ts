import {isPending, isSaved} from './workflowIdentity.ts';
import type {WorkflowEvent} from './WorkflowStore.ts';

const displayNameOf = (row: WorkflowEvent): string => row.display_name || row.workflow_event || row.event_id || '';

/**
 * Re-attaches browser-only pending clones to a freshly fetched server list.
 * The server only knows about saved workflows and placeholders, so without this
 * a refresh triggered by saving or deleting some *other* workflow would silently
 * discard an unsaved clone.
 */
export function mergePendingClones(serverRows: WorkflowEvent[], pendingRows: WorkflowEvent[]): WorkflowEvent[] {
  if (!pendingRows.length) return serverRows;

  const merged = [...serverRows];
  for (const pending of pendingRows) {
    // Keep the clone next to the workflow it came from, falling back to the end
    // of its event-type group once that workflow has been deleted.
    let at = merged.findIndex((row) => row.event_id === pending._clonedFromEventId);
    if (at < 0) at = merged.findLastIndex((row) => row.workflow_event === pending.workflow_event);
    if (at < 0) merged.push(pending);
    else merged.splice(at + 1, 0, pending);
  }
  return merged;
}

/**
 * Numbers workflows that share an event type, returning a name per `event_id`.
 * Computing the whole list at once keeps this O(n) and, unlike deriving it per
 * row, does not require cloning rows to carry the derived name.
 */
export function buildDisplayNames(rows: WorkflowEvent[]): Map<string, string> {
  const groups = new Map<string, WorkflowEvent[]>();
  for (const row of rows) {
    const groupKey = row.workflow_event || row.event_id;
    const group = groups.get(groupKey);
    if (group) group.push(row);
    else groups.set(groupKey, [row]);
  }

  const names = new Map<string, string>();
  for (const group of groups.values()) {
    // Saved workflows are numbered ahead of pending clones, so starting a clone
    // never renumbers the rows above it.
    const ordered = group.filter((row) => !isPending(row)).concat(group.filter((row) => isPending(row)));
    for (const [index, row] of ordered.entries()) {
      const baseName = displayNameOf(row);
      names.set(row.event_id, ordered.length > 1 ? `${baseName} #${index + 1}` : baseName);
    }
  }
  return names;
}

/** Colour class for the status dot shown next to a sidebar row. */
export function statusClass(row: WorkflowEvent): string {
  if (!isSaved(row)) return 'status-inactive';
  return row.enabled === false ? 'status-disabled' : 'status-active';
}
