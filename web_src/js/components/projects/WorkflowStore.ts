import {reactive} from 'vue';
import {GET, POST} from '../../modules/fetch.ts';
import {showErrorToast} from '../../modules/toast.ts';
import camelcaseKeys from 'camelcase-keys';

type WorkflowFilters = {
  issueType: string;
  sourceColumn: string;
  targetColumn: string;
  labels: string[];
};

type WorkflowIssueStateAction = '' | 'close' | 'reopen';

type WorkflowActions = {
  column: string;
  addLabels: string[];
  removeLabels: string[];
  issueState: WorkflowIssueStateAction;
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
  availableFilters?: string[];
  availableActions?: string[];
};

export type WorkflowEvent = {
  id: number;
  eventId: string;
  workflowEvent?: string;
  displayName?: string;
  summary?: string;
  enabled?: boolean;
  capabilities?: WorkflowCapabilities;
  filters?: Array<{type: string, value: string}>;
  actions?: Array<{type: string, value: string}>;
  _isEditing?: boolean;
  isConfigured?: boolean;
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
  getDraft(eventId: string): WorkflowDraftState | undefined;
  updateDraft(eventId: string, filters: WorkflowFilters, actions: WorkflowActions): void;
  clearDraft(eventId: string): void;
  loadEvents(): Promise<WorkflowEvent[]>;
  loadProjectColumns(): Promise<void>;
  loadWorkflowData(eventId: string): Promise<void>;
  loadProjectLabels(): Promise<void>;
  resetWorkflowData(): void;
  saveWorkflow(): Promise<void>;
  saveWorkflowStatus(): Promise<void>;
  deleteWorkflow(): Promise<void>;
};

const createDefaultFilters = (): WorkflowFilters => ({issueType: '', sourceColumn: '', targetColumn: '', labels: []});
const createDefaultActions = (): WorkflowActions => ({column: '', addLabels: [], removeLabels: [], issueState: ''});

const camelToSnake = (key: string): string => key.replace(/([A-Z])/g, '_$1').toLowerCase();

function convertKeysToSnakeCase<T extends Record<string, unknown>>(obj: T): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(obj)) {
    result[camelToSnake(key)] = value;
  }
  return result;
}

function convertFilters(workflow: any): WorkflowFilters {
  const filters = createDefaultFilters();
  if (workflow?.filters && Array.isArray(workflow.filters)) {
    for (const filter of workflow.filters) {
      if (filter.type === 'issue_type') {
        filters.issueType = filter.value;
      } else if (filter.type === 'source_column') {
        filters.sourceColumn = filter.value;
      } else if (filter.type === 'target_column') {
        filters.targetColumn = filter.value;
      } else if (filter.type === 'labels') {
        filters.labels.push(filter.value);
      }
    }
  }
  return filters;
}

function convertActions(workflow: any): WorkflowActions {
  const actions = createDefaultActions();

  if (workflow?.actions && Array.isArray(workflow.actions)) {
    for (const action of workflow.actions) {
      if (action.type === 'column') {
        // Backend returns string, keep as string to match column.id type
        actions.column = action.value;
      } else if (action.type === 'add_labels') {
        // Backend returns string, keep as string to match label.id type
        actions.addLabels.push(action.value);
      } else if (action.type === 'remove_labels') {
        // Backend returns string, keep as string to match label.id type
        actions.removeLabels.push(action.value);
      } else if (action.type === 'issue_state') {
        actions.issueState = action.value as WorkflowIssueStateAction;
      }
    }
  }
  return actions;
}

const cloneFilters = (filters: WorkflowFilters): WorkflowFilters => ({
  issueType: filters.issueType,
  sourceColumn: filters.sourceColumn,
  targetColumn: filters.targetColumn,
  labels: Array.from(filters.labels),
});

const cloneActions = (actions: WorkflowActions): WorkflowActions => ({
  column: actions.column,
  addLabels: Array.from(actions.addLabels),
  removeLabels: Array.from(actions.removeLabels),
  issueState: actions.issueState,
});

export function createWorkflowStore(props: any): WorkflowStoreState {
  const store: WorkflowStoreState = reactive<WorkflowStoreState>({
    workflowEvents: [] as WorkflowEvent[],
    selectedItem: props.eventID,
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

    getDraft(eventId: string): WorkflowDraftState | undefined {
      return store.workflowDrafts[eventId];
    },

    updateDraft(eventId: string, filters: WorkflowFilters, actions: WorkflowActions) {
      store.workflowDrafts[eventId] = {
        filters: cloneFilters(filters),
        actions: cloneActions(actions),
      };
    },

    clearDraft(eventId: string) {
      delete store.workflowDrafts[eventId];
    },

    async loadEvents(): Promise<WorkflowEvent[]> {
      const response = await GET(`${props.projectLink}/workflows/events`);
      const data = await response.json();
      store.workflowEvents = camelcaseKeys(data, {deep: true}) as WorkflowEvent[];
      return store.workflowEvents;
    },

    async loadProjectColumns(): Promise<void> {
      try {
        const response = await GET(`${props.projectLink}/workflows/columns`);
        store.projectColumns = await response.json() as ProjectColumn[];
      } catch (error) {
        console.error('Failed to load project columns:', error);
        store.projectColumns = [];
      }
    },

    async loadWorkflowData(eventId: string): Promise<void> {
      store.loading = true;
      try {
        // Load project columns and labels for the dropdowns
        await store.loadProjectColumns();
        await store.loadProjectLabels();

        const draft = store.getDraft(eventId);
        if (draft) {
          store.workflowFilters = cloneFilters(draft.filters);
          store.workflowActions = cloneActions(draft.actions);
          return;
        }

        // Find the workflow from existing workflowEvents
        const workflow = store.workflowEvents.find((e: WorkflowEvent) => e.eventId === eventId);

        store.workflowFilters = convertFilters(workflow);
        store.workflowActions = convertActions(workflow);
        store.updateDraft(eventId, store.workflowFilters, store.workflowActions);
      } finally {
        store.loading = false;
      }
    },

    async loadProjectLabels(): Promise<void> {
      try {
        const response = await GET(`${props.projectLink}/workflows/labels`);
        store.projectLabels = await response.json() as ProjectLabel[];
      } catch (error) {
        console.error('Failed to load project labels:', error);
        store.projectLabels = [];
      }
    },

    resetWorkflowData(): void {
      store.workflowFilters = createDefaultFilters();
      store.workflowActions = createDefaultActions();

      const currentEventId = store.selectedWorkflow?.eventId;
      if (currentEventId) {
        store.updateDraft(currentEventId, store.workflowFilters, store.workflowActions);
      }
    },

    async saveWorkflow(): Promise<void> {
      if (!store.selectedWorkflow) return;

      // Validate: at least one action must be configured
      const hasAtLeastOneAction = Boolean(
        store.workflowActions.column ||
        store.workflowActions.addLabels.length > 0 ||
        store.workflowActions.removeLabels.length > 0 ||
        store.workflowActions.issueState,
      );

      if (!hasAtLeastOneAction) {
        showErrorToast(props.locale.atLeastOneActionRequired);
        return;
      }

      store.saving = true;
      try {
        // For new workflows, use the base event type
        const eventId = store.selectedWorkflow.eventId;

        // Convert frontend data format to backend JSON format
        const postData = {
          event_id: eventId,
          filters: convertKeysToSnakeCase(store.workflowFilters),
          actions: convertKeysToSnakeCase(store.workflowActions),
        };

        const response = await POST(`${props.projectLink}/workflows/${eventId}`, {
          data: postData,
          headers: {
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          let errorMessage = `${props.locale.failedToSaveWorkflow}: ${response.status} ${response.statusText}`;
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

        const data = await response.json();
        const result = camelcaseKeys(data, {deep: true});
        if (result.success && result.workflow) {
          // Always reload the events list to get the updated structure
          // This ensures we have both the base event and the new filtered event
          const eventKey = typeof store.selectedWorkflow.eventId === 'string' ? store.selectedWorkflow.eventId : '';
          const wasNewWorkflow = store.selectedWorkflow.id === 0 ||
                                 eventKey.startsWith('new-') ||
                                 eventKey.startsWith('clone-');

          if (wasNewWorkflow && store.selectedWorkflow.workflowEvent) {
            store.clearDraft(store.selectedWorkflow.workflowEvent);
          }

          // Reload events from server to get the correct event structure
          await store.loadEvents();

          // Find the reloaded workflow which has complete data including capabilities
          const reloadedWorkflow = store.workflowEvents.find((w: WorkflowEvent) => w.eventId === result.workflow.eventId);

          if (reloadedWorkflow) {
            // Use the reloaded workflow as it has all the necessary fields
            store.selectedWorkflow = reloadedWorkflow;
            store.selectedItem = reloadedWorkflow.eventId;
          } else {
            // Fallback: use the result from backend (shouldn't normally happen)
            store.selectedWorkflow = result.workflow;
            store.selectedItem = result.workflow.eventId;
          }

          store.workflowFilters = convertFilters(store.selectedWorkflow);
          store.workflowActions = convertActions(store.selectedWorkflow);
          if (store.selectedWorkflow?.eventId) {
            store.updateDraft(store.selectedWorkflow.eventId, store.workflowFilters, store.workflowActions);
          }

          // Update URL to use the new workflow ID
          if (wasNewWorkflow && store.selectedWorkflow?.eventId) {
            const newUrl = `${props.projectLink}/workflows/${store.selectedWorkflow.eventId}`;
            window.history.replaceState({eventId: store.selectedWorkflow.eventId}, '', newUrl);
          }
        } else {
          console.error('Unexpected response format:', result);
          showErrorToast(`${props.locale.failedToSaveWorkflow}: Unexpected response format`);
        }
      } catch (error) {
        console.error('Failed to save workflow:', error);
        showErrorToast(`${props.locale.failedToSaveWorkflow}: ${error.message}`);
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
          showErrorToast(`${props.locale.failedToUpdateWorkflowStatus}: ${response.status} ${response.statusText}`);
          // Revert the status change on error
          selected.enabled = previousEnabled;
          return;
        }

        const result = await response.json();
        if (result.success) {
          // Update workflow in the list
          const existingIndex = store.workflowEvents.findIndex((e: WorkflowEvent) => e.eventId === selected.eventId);
          if (existingIndex >= 0) {
            store.workflowEvents[existingIndex].enabled = desiredEnabled;
          }
        } else {
          // Revert the status change on failure
          selected.enabled = previousEnabled;
          showErrorToast(`${props.locale.failedToUpdateWorkflowStatus}: Unexpected error`);
        }
      } catch (error) {
        console.error('Failed to update workflow status:', error);
        // Revert the status change on error
        selected.enabled = previousEnabled;
        showErrorToast(`${props.locale.failedToUpdateWorkflowStatus}: ${error.message}`);
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
          showErrorToast(`${props.locale.failedToDeleteWorkflow}: ${response.status} ${response.statusText}`);
          return;
        }

        const result = await response.json();
        if (result.success) {
          // Remove workflow from the list
          const existingIndex = store.workflowEvents.findIndex((e: WorkflowEvent) => e.eventId === selected.eventId);
          if (existingIndex >= 0) {
            store.workflowEvents.splice(existingIndex, 1);
          }
        } else {
          showErrorToast(`${props.locale.failedToDeleteWorkflow}: Unexpected error`);
        }
      } catch (error) {
        console.error('Error deleting workflow:', error);
        showErrorToast(`${props.locale.failedToDeleteWorkflow}: ${error.message}`);
      }
    },

  });
  return store;
}
