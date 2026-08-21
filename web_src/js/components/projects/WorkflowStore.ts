import {reactive} from 'vue';
import {GET, POST} from '../../modules/fetch.ts';
import {showErrorToast} from '../../modules/toast.ts';
import {isPending, saveTargetId} from './workflowIdentity.ts';
import {mergePendingClones} from './workflowList.ts';
import type {WorkflowLocale} from './workflowLocale.ts';

// Minimum props the store needs from the Vue component
type StoreProps = {
  projectLink: string;
  locale: Pick<WorkflowLocale,
    'atLeastOneActionRequired' | 'saveWorkflowFailed' | 'updateWorkflowFailed' |
    'deleteWorkflowFailed' | 'unexpectedResponseFormat' | 'unexpectedError'>;
};

type WorkflowFilters = {
  issue_type: string;
  source_column: string;
  target_column: string;
  labels: string[];
};

type WorkflowIssueStateAction = '' | 'close' | 'reopen';

type WorkflowActions = {
  column: string;
  add_labels: string[];
  remove_labels: string[];
  issue_state: WorkflowIssueStateAction;
};

type WorkflowDraftState = {
  filters: WorkflowFilters;
  actions: WorkflowActions;
};

export type ProjectColumn = {
  id: number;
  title: string;
};

export type ProjectLabel = {
  id: number;
  name: string;
  color: string;
};

type WorkflowCapabilities = {
  available_filters?: string[];
  available_actions?: string[];
};

export type WorkflowEvent = {
  id: number;
  event_id: string;
  workflow_event?: string;
  display_name?: string;
  summary?: string;
  enabled?: boolean;
  capabilities?: WorkflowCapabilities;
  filters?: Array<{type: string, value: string}>;
  actions?: Array<{type: string, value: string}>;
  _clonedFromEventId?: string;
  is_configured?: boolean;
} & Record<string, unknown>;

export type WorkflowStoreState = {
  workflowEvents: WorkflowEvent[];
  // Derived from selectedWorkflow, never assigned: a separately stored key drifted
  // out of step with the loaded row, leaving the sidebar highlighting a workflow
  // whose editor pane was empty.
  readonly selectedItem: string | null;
  selectedWorkflow: WorkflowEvent | null;
  projectColumns: ProjectColumn[];
  projectLabels: ProjectLabel[];
  saving: boolean;
  loading: boolean;
  workflowFilters: WorkflowFilters;
  workflowActions: WorkflowActions;
  workflowDrafts: Record<string, WorkflowDraftState>;
  getDraft(event_id: string): WorkflowDraftState | undefined;
  updateDraft(event_id: string, filters: WorkflowFilters, actions: WorkflowActions): void;
  clearDraft(event_id: string): void;
  removePendingRow(event_id: string): void;
  loadEvents(): Promise<WorkflowEvent[]>;
  loadProjectOptions(): Promise<void>;
  loadWorkflowData(event_id: string): Promise<void>;
  saveWorkflow(): Promise<boolean>;
  saveWorkflowStatus(desiredEnabled: boolean): Promise<void>;
  deleteWorkflow(): Promise<void>;
};

const createDefaultFilters = (): WorkflowFilters => ({issue_type: '', source_column: '', target_column: '', labels: []});
const createDefaultActions = (): WorkflowActions => ({column: '', add_labels: [], remove_labels: [], issue_state: ''});

const getErrorMessage = (error: unknown): string => error instanceof Error ? error.message : String(error);

function convertFilters(workflow?: WorkflowEvent | null): WorkflowFilters {
  const filters = createDefaultFilters();
  if (workflow?.filters && Array.isArray(workflow.filters)) {
    for (const filter of workflow.filters) {
      if (filter.type === 'issue_type') {
        filters.issue_type = filter.value;
      } else if (filter.type === 'source_column') {
        filters.source_column = filter.value;
      } else if (filter.type === 'target_column') {
        filters.target_column = filter.value;
      } else if (filter.type === 'labels') {
        filters.labels.push(filter.value);
      }
    }
  }
  return filters;
}

function convertActions(workflow?: WorkflowEvent | null): WorkflowActions {
  const actions = createDefaultActions();

  if (workflow?.actions && Array.isArray(workflow.actions)) {
    for (const action of workflow.actions) {
      if (action.type === 'column') {
        // Backend returns string, keep as string to match column.id type
        actions.column = action.value;
      } else if (action.type === 'add_labels') {
        // Backend returns string, keep as string to match label.id type
        actions.add_labels.push(action.value);
      } else if (action.type === 'remove_labels') {
        // Backend returns string, keep as string to match label.id type
        actions.remove_labels.push(action.value);
      } else if (action.type === 'issue_state') {
        actions.issue_state = action.value as WorkflowIssueStateAction;
      }
    }
  }
  return actions;
}

const cloneFilters = (filters: WorkflowFilters): WorkflowFilters => ({
  issue_type: filters.issue_type,
  source_column: filters.source_column,
  target_column: filters.target_column,
  labels: Array.from(filters.labels),
});

const cloneActions = (actions: WorkflowActions): WorkflowActions => ({
  column: actions.column,
  add_labels: Array.from(actions.add_labels),
  remove_labels: Array.from(actions.remove_labels),
  issue_state: actions.issue_state,
});

export function createWorkflowStore(props: StoreProps): WorkflowStoreState {
  const store: WorkflowStoreState = reactive<WorkflowStoreState>({
    workflowEvents: [] as WorkflowEvent[],
    get selectedItem(): string | null {
      return store.selectedWorkflow?.event_id ?? null;
    },
    selectedWorkflow: null,
    projectColumns: [],
    projectLabels: [],
    saving: false,
    loading: false,
    workflowFilters: createDefaultFilters(),
    workflowActions: createDefaultActions(),

    workflowDrafts: {},

    getDraft: (event_id: string): WorkflowDraftState | undefined => store.workflowDrafts[event_id],

    updateDraft(event_id: string, filters: WorkflowFilters, actions: WorkflowActions) {
      store.workflowDrafts[event_id] = {
        filters: cloneFilters(filters),
        actions: cloneActions(actions),
      };
    },

    clearDraft(event_id: string) {
      delete store.workflowDrafts[event_id];
    },

    removePendingRow(event_id: string) {
      const at = store.workflowEvents.findIndex((e: WorkflowEvent) => e.event_id === event_id);
      if (at >= 0) store.workflowEvents.splice(at, 1);
    },

    async loadEvents(): Promise<WorkflowEvent[]> {
      const response = await GET(`${props.projectLink}/workflows/events`);
      const data = await response.json() as WorkflowEvent[];
      // The response only contains saved workflows and placeholders, so pending
      // clones are re-attached: a reload triggered by saving or deleting some
      // other workflow must not discard unsaved work.
      store.workflowEvents = mergePendingClones(data, store.workflowEvents.filter((e: WorkflowEvent) => isPending(e)));
      return store.workflowEvents;
    },

    async loadProjectOptions(): Promise<void> {
      try {
        const response = await GET(`${props.projectLink}/workflows/options`);
        const data = await response.json();
        store.projectColumns = data.columns as ProjectColumn[];
        store.projectLabels = data.labels as ProjectLabel[];
      } catch (error) {
        console.error('Failed to load project columns and labels:', error);
        store.projectColumns = [];
        store.projectLabels = [];
      }
    },

    async loadWorkflowData(event_id: string): Promise<void> {
      store.loading = true;
      try {
        const draft = store.getDraft(event_id);
        if (draft) {
          store.workflowFilters = cloneFilters(draft.filters);
          store.workflowActions = cloneActions(draft.actions);
          return;
        }

        // Find the workflow from existing workflowEvents
        const workflow = store.workflowEvents.find((e: WorkflowEvent) => e.event_id === event_id);

        store.workflowFilters = convertFilters(workflow);
        store.workflowActions = convertActions(workflow);
        store.updateDraft(event_id, store.workflowFilters, store.workflowActions);
      } finally {
        store.loading = false;
      }
    },

    async saveWorkflow(): Promise<boolean> {
      if (!store.selectedWorkflow) return false;

      // Validate: at least one action must be configured
      const hasAtLeastOneAction = Boolean(
        store.workflowActions.column ||
        store.workflowActions.add_labels.length > 0 ||
        store.workflowActions.remove_labels.length > 0 ||
        store.workflowActions.issue_state,
      );

      if (!hasAtLeastOneAction) {
        showErrorToast(props.locale.atLeastOneActionRequired);
        return false;
      }

      store.saving = true;
      try {
        const event_id = saveTargetId(store.selectedWorkflow);

        const postData = {
          event_id,
          filters: store.workflowFilters,
          actions: store.workflowActions,
        };

        const response = await POST(`${props.projectLink}/workflows/${event_id}`, {
          data: postData,
          headers: {
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          let errorMessage = `${props.locale.saveWorkflowFailed}: ${response.status} ${response.statusText}`;
          try {
            const errorData = await response.json();
            if (errorData.errorMessage) {
              errorMessage = errorData.errorMessage;
            }
          } catch {
            const errorText = await response.text();
            console.error('Response error:', errorText);
            errorMessage += `\n${errorText}`;
          }
          showErrorToast(errorMessage);
          return false;
        }

        const result = await response.json();
        if (result.success && result.workflow) {
          const wasNewWorkflow = store.selectedWorkflow.id === 0;
          if (wasNewWorkflow) {
            // The row just persisted is no longer pending, so drop it (and its
            // draft) before reloading rather than letting it be carried over.
            store.clearDraft(store.selectedWorkflow.event_id);
            store.removePendingRow(store.selectedWorkflow.event_id);
          }

          await store.loadEvents();

          const reloadedWorkflow = store.workflowEvents.find((w: WorkflowEvent) => w.event_id === result.workflow.event_id);
          const selected = reloadedWorkflow ?? {...result.workflow, is_configured: true} satisfies WorkflowEvent;
          store.selectedWorkflow = selected;

          store.workflowFilters = convertFilters(selected);
          store.workflowActions = convertActions(selected);
          store.updateDraft(selected.event_id, store.workflowFilters, store.workflowActions);

          if (wasNewWorkflow && selected.event_id) {
            const newUrl = `${props.projectLink}/workflows/${selected.event_id}`;
            window.history.replaceState({event_id: selected.event_id}, '', newUrl);
          }
          return true;
        }
        console.error('Unexpected response format:', result);
        showErrorToast(`${props.locale.saveWorkflowFailed}: ${props.locale.unexpectedResponseFormat}`);
        return false;
      } catch (error) {
        console.error('Failed to save workflow:', error);
        showErrorToast(`${props.locale.saveWorkflowFailed}: ${getErrorMessage(error)}`);
        return false;
      } finally {
        store.saving = false;
      }
    },

    async saveWorkflowStatus(desiredEnabled: boolean): Promise<void> {
      const selected = store.selectedWorkflow;
      if (!selected || selected.id === 0) return;

      const previousEnabled = Boolean(selected.enabled);
      selected.enabled = desiredEnabled;

      try {
        const formData = new FormData();
        formData.append('enabled', desiredEnabled.toString());

        const response = await POST(`${props.projectLink}/workflows/${selected.id}/status`, {
          data: formData,
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error('Failed to update workflow status:', errorText);
          showErrorToast(`${props.locale.updateWorkflowFailed}: ${response.status} ${response.statusText}`);
          // Revert the status change on error
          selected.enabled = previousEnabled;
          return;
        }

        const result = await response.json();
        if (result.success) {
          // Update workflow in the list
          const existingIndex = store.workflowEvents.findIndex((e: WorkflowEvent) => e.event_id === selected.event_id);
          if (existingIndex >= 0) {
            store.workflowEvents[existingIndex].enabled = desiredEnabled;
          }
        } else {
          // Revert the status change on failure
          selected.enabled = previousEnabled;
          showErrorToast(`${props.locale.updateWorkflowFailed}: ${props.locale.unexpectedError}`);
        }
      } catch (error) {
        console.error('Failed to update workflow status:', error);
        // Revert the status change on error
        selected.enabled = previousEnabled;
        showErrorToast(`${props.locale.updateWorkflowFailed}: ${getErrorMessage(error)}`);
      }
    },

    async deleteWorkflow(): Promise<void> {
      const selected = store.selectedWorkflow;
      if (!selected || selected.id === 0) return;

      try {
        const response = await POST(`${props.projectLink}/workflows/${selected.id}/delete`, {
          data: new FormData(),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error('Failed to delete workflow:', errorText);
          showErrorToast(`${props.locale.deleteWorkflowFailed}: ${response.status} ${response.statusText}`);
          return;
        }

        // Remove workflow from the list
        const existingIndex = store.workflowEvents.findIndex((e: WorkflowEvent) => e.event_id === selected.event_id);
        if (existingIndex >= 0) {
          store.workflowEvents.splice(existingIndex, 1);
        }
      } catch (error) {
        console.error('Error deleting workflow:', error);
        showErrorToast(`${props.locale.deleteWorkflowFailed}: ${getErrorMessage(error)}`);
      }
    },

  });
  return store;
}
