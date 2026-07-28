import {onMounted, onUnmounted} from 'vue';
import {isPendingKey, urlKeyFor} from './workflowIdentity.ts';
import type {WorkflowEvent} from './WorkflowStore.ts';

/**
 * Owns the address bar: which workflow the URL names, and reacting to Back.
 * Only server-addressable keys are ever written, so every history entry names a
 * workflow that still exists after a reload.
 */
export function useWorkflowRouting(projectLink: string) {
  const workflowsUrl = `${projectLink}/workflows`;
  let onNavigate: ((eventID: string) => void) | null = null;

  // A pending clone has no server-side address, so urlKeyFor reports the workflow
  // it was cloned from: reloading then lands on a workflow that exists rather
  // than silently selecting an unrelated one.
  const urlFor = (row: WorkflowEvent): string => {
    const key = urlKeyFor(row);
    return key ? `${workflowsUrl}/${key}` : workflowsUrl;
  };

  const pushSelection = (row: WorkflowEvent) => {
    const url = urlFor(row);
    const state = {event_id: urlKeyFor(row)};
    // Selecting whatever the URL already names must not stack a duplicate entry.
    // This covers the initial load, starting a clone (which keeps the source's
    // URL) and responding to Back, where pushing would strand the user.
    if (new URL(url, window.location.origin).pathname === window.location.pathname) {
      window.history.replaceState(state, '', url);
    } else {
      window.history.pushState(state, '', url);
    }
  };

  const resetUrl = () => {
    window.history.pushState({}, '', workflowsUrl);
  };

  /** The deep link to honour on load, or null when it names nothing addressable. */
  const routableKey = (eventID: string): string | null => {
    if (!eventID || isPendingKey(eventID)) return null;
    return eventID;
  };

  const popstateHandler = (e: PopStateEvent) => {
    const eventID = e.state?.event_id;
    // History written before pending keys were kept out of the URL can still
    // hold one; it resolves to nothing, so ignore it rather than half-navigate.
    if (!eventID || isPendingKey(eventID)) return;
    onNavigate?.(eventID);
  };

  onMounted(() => window.addEventListener('popstate', popstateHandler));
  onUnmounted(() => window.removeEventListener('popstate', popstateHandler));

  return {
    urlFor,
    pushSelection,
    resetUrl,
    routableKey,
    /** Wired once selection exists, so neither module has to import the other. */
    handleNavigation(callback: (eventID: string) => void) {
      onNavigate = callback;
    },
  };
}
