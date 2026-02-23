import {reactive} from 'vue';
import {GET, POST} from '../../modules/fetch.ts';
import {showErrorToast} from '../../modules/toast.ts';

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
  id: string | number;
  title: string;
};

export type ProjectLabel = {
  id: string | number;
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
  _isEditing?: boolean;
  _clonedFromEventId?: string;
  is_configured?: boolean;
} & Record<string, unknown>;

type WorkflowStoreState = {
  workflowEvents: WorkflowEvent[];
  selectedItem: string | null;
  selectedWorkflow: WorkflowEvent | null;
  projectColumns: ProjectColumn[];
  projectLabels: ProjectLabel[];
  saving: boolean;
  loading: boolean;
  showCreateDialog: boolean;
  selectedEventType: string | null;
  workflowFilters: WorkflowFilters;
  workflowActions: WorkflowActions;
  workflowDrafts: Record<string, WorkflowDraftState>;
  getDraft(event_id: string): WorkflowDraftState | undefined;
  updateDraft(event_id: string, filters: WorkflowFilters, actions: WorkflowActions): void;
  clearDraft(event_id: string): void;
  loadEvents(): Promise<WorkflowEvent[]>;
  loadProjectOptions(): Promise<void>;
  loadWorkflowData(event_id: string): Promise<void>;
  resetWorkflowData(): void;
  saveWorkflow(): Promise<void>;
  saveWorkflowStatus(): Promise<void>;
  deleteWorkflow(): Promise<void>;
};

const createDefaultFilters = (): WorkflowFilters => ({issue_type: '', source_column: '', target_column: '', labels: []});
const createDefaultActions = (): WorkflowActions => ({column: '', add_labels: [], remove_labels: [], issue_state: ''});

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

export function createWorkflowStore(props: any): WorkflowStoreState {
  const store: WorkflowStoreState = reactive<WorkflowStoreState>({
    workflowEvents: [] as WorkflowEvent[],
    selectedItem: props.event_id,
    selectedWorkflow: null,
    projectColumns: [],
    projectLabels: [], // Add labels data
    saving: false,
    loading: false, // Add loading state to prevent rapid clicks
    showCreateDialog: false, // For create workflow dialog
    selectedEventType: null, // For workflow creation

    workflowFilters: createDefaultFilters(),
    workflowActions: createDefaultActions(),

    workflowDrafts: {} as Record<string, WorkflowDraftState>,

    getDraft(event_id: string): WorkflowDraftState | undefined {
      return store.workflowDrafts[event_id];
    },

    updateDraft(event_id: string, filters: WorkflowFilters, actions: WorkflowActions) {
      store.workflowDrafts[event_id] = {
        filters: cloneFilters(filters),
        actions: cloneActions(actions),
      };
    },

    clearDraft(event_id: string) {
      delete store.workflowDrafts[event_id];
    },

    async loadEvents(): Promise<WorkflowEvent[]> {
      const response = await GET(`${props.projectLink}/workflows/events`);
      const data = await response.json();
      store.workflowEvents = data as WorkflowEvent[];
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
        // Load project columns and labels for the dropdowns
        await store.loadProjectOptions();

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

    resetWorkflowData(): void {
      store.workflowFilters = createDefaultFilters();
      store.workflowActions = createDefaultActions();

      const currentevent_id = store.selectedWorkflow?.event_id;
      if (currentevent_id) {
        store.updateDraft(currentevent_id, store.workflowFilters, store.workflowActions);
      }
    },

    async saveWorkflow(): Promise<void> {
      if (!store.selectedWorkflow) return;

      // Validate: at least one action must be configured
      const hasAtLeastOneAction = Boolean(
        store.workflowActions.column ||
        store.workflowActions.add_labels.length > 0 ||
        store.workflowActions.remove_labels.length > 0 ||
        store.workflowActions.issue_state,
      );

      if (!hasAtLeastOneAction) {
        showErrorToast(props.locale.atLeastOneActionRequired);
        return;
      }

      store.saving = true;
      try {
        // For new workflows, use the base event type
        const event_id = store.selectedWorkflow.event_id;

        // Convert frontend data format to backend JSON format
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
          return;
        }

        const result = await response.json();
        if (result.success && result.workflow) {
          // Always reload the events list to get the updated structure
          // This ensures we have both the base event and the new filtered event
          const eventKey = typeof store.selectedWorkflow.event_id === 'string' ? store.selectedWorkflow.event_id : '';
          const wasNewWorkflow = store.selectedWorkflow.id === 0 ||
                                 eventKey.startsWith('new-') ||
                                 eventKey.startsWith('clone-');

          if (wasNewWorkflow && store.selectedWorkflow.workflow_event) {
            store.clearDraft(store.selectedWorkflow.workflow_event);
          }

          // Reload events from server to get the correct event structure
          await store.loadEvents();

          // Find the reloaded workflow which has complete data including capabilities
          const reloadedWorkflow = store.workflowEvents.find((w: WorkflowEvent) => w.event_id === result.workflow.event_id);

          if (reloadedWorkflow) {
            // Use the reloaded workflow as it has all the necessary fields
            store.selectedWorkflow = reloadedWorkflow;
            store.selectedItem = reloadedWorkflow.event_id;
          } else {
            // Fallback: use the result from backend (shouldn't normally happen)
            store.selectedWorkflow = result.workflow;
            store.selectedItem = result.workflow.event_id;
          }

          store.workflowFilters = convertFilters(store.selectedWorkflow);
          store.workflowActions = convertActions(store.selectedWorkflow);
          if (store.selectedWorkflow?.event_id) {
            store.updateDraft(store.selectedWorkflow.event_id, store.workflowFilters, store.workflowActions);
          }

          // Update URL to use the new workflow ID
          if (wasNewWorkflow && store.selectedWorkflow?.event_id) {
            const newUrl = `${props.projectLink}/workflows/${store.selectedWorkflow.event_id}`;
            window.history.replaceState({event_id: store.selectedWorkflow.event_id}, '', newUrl);
          }
        } else {
          console.error('Unexpected response format:', result);
          showErrorToast(`${props.locale.saveWorkflowFailed}: Unexpected response format`);
        }
      } catch (error) {
        console.error('Failed to save workflow:', error);
        showErrorToast(`${props.locale.saveWorkflowFailed}: ${error.message}`);
      } finally {
        store.saving = false;
      }
    },

    async saveWorkflowStatus(): Promise<void> {
      const selected = store.selectedWorkflow;
      if (!selected || selected.id === 0) return;

      const desiredEnabled = Boolean(selected.enabled);
      const previousEnabled = !desiredEnabled;

      try {
        const formData = new FormData();
        formData.append('enabled', desiredEnabled.toString());

        // Use workflow ID for status update
        const workflowId = selected.id;
        const response = await POST(`${props.projectLink}/workflows/${workflowId}/status`, {
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
          showErrorToast(`${props.locale.updateWorkflowFailed}: Unexpected error`);
        }
      } catch (error) {
        console.error('Failed to update workflow status:', error);
        // Revert the status change on error
        selected.enabled = previousEnabled;
        showErrorToast(`${props.locale.updateWorkflowFailed}: ${error.message}`);
      }
    },

    async deleteWorkflow(): Promise<void> {
      const selected = store.selectedWorkflow;
      if (!selected || selected.id === 0) return;

      try {
        // Use workflow ID for deletion
        const workflowId = selected.id;
        const response = await POST(`${props.projectLink}/workflows/${workflowId}/delete`, {
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
        showErrorToast(`${props.locale.deleteWorkflowFailed}: ${error.message}`);
      }
    },

  });
  return store;
}
