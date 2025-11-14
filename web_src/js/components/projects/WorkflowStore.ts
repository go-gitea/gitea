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

const createDefaultFilters = (): WorkflowFilters => ({issue_type: '', source_column: '', target_column: '', labels: []});
const createDefaultActions = (): WorkflowActions => ({column: '', add_labels: [], remove_labels: [], issue_state: ''});

function convertFilters(workflow: any): WorkflowFilters {
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

function convertActions(workflow: any): WorkflowActions {
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

export function createWorkflowStore(props: any) {
  const store = reactive({
    workflowEvents: [],
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

    async loadEvents() {
      const response = await GET(`${props.projectLink}/workflows/events`);
      store.workflowEvents = await response.json();
      return store.workflowEvents;
    },

    async loadProjectColumns() {
      try {
        const response = await GET(`${props.projectLink}/workflows/columns`);
        store.projectColumns = await response.json();
      } catch (error) {
        console.error('Failed to load project columns:', error);
        store.projectColumns = [];
      }
    },

    async loadWorkflowData(eventId: string) {
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
        const workflow = store.workflowEvents.find((e) => e.event_id === eventId);

        store.workflowFilters = convertFilters(workflow);
        store.workflowActions = convertActions(workflow);
        store.updateDraft(eventId, store.workflowFilters, store.workflowActions);
      } finally {
        store.loading = false;
      }
    },

    async loadProjectLabels() {
      try {
        const response = await GET(`${props.projectLink}/workflows/labels`);
        store.projectLabels = await response.json();
      } catch (error) {
        console.error('Failed to load project labels:', error);
        store.projectLabels = [];
      }
    },

    resetWorkflowData() {
      store.workflowFilters = createDefaultFilters();
      store.workflowActions = createDefaultActions();

      const currentEventId = store.selectedWorkflow?.event_id;
      if (currentEventId) {
        store.updateDraft(currentEventId, store.workflowFilters, store.workflowActions);
      }
    },

    async saveWorkflow() {
      if (!store.selectedWorkflow) return;

      // Validate: at least one action must be configured
      const hasAtLeastOneAction = Boolean(
        store.workflowActions.column ||
        store.workflowActions.add_labels.length > 0 ||
        store.workflowActions.remove_labels.length > 0 ||
        store.workflowActions.issue_state,
      );

      if (!hasAtLeastOneAction) {
        showErrorToast(props.locale.atLeastOneActionRequired || 'At least one action must be configured');
        return;
      }

      store.saving = true;
      try {
        // For new workflows, use the base event type
        const eventId = store.selectedWorkflow.event_id;

        // Convert frontend data format to backend JSON format
        const postData = {
          event_id: eventId,
          filters: store.workflowFilters,
          actions: store.workflowActions,
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
            if (errorData.message) {
              errorMessage = errorData.message;
            } else if (errorData.error === 'NoActions') {
              errorMessage = props.locale.atLeastOneActionRequired || 'At least one action must be configured';
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

          if (wasNewWorkflow) {
            store.clearDraft(store.selectedWorkflow.workflow_event);
          }

          // Reload events from server to get the correct event structure
          await store.loadEvents();

          // Find the reloaded workflow which has complete data including capabilities
          const reloadedWorkflow = store.workflowEvents.find((w) => w.event_id === result.workflow.event_id);

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
          if (wasNewWorkflow) {
            const newUrl = `${props.projectLink}/workflows/${store.selectedWorkflow.event_id}`;
            window.history.replaceState({eventId: store.selectedWorkflow.event_id}, '', newUrl);
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

    async saveWorkflowStatus() {
      if (!store.selectedWorkflow || store.selectedWorkflow.id === 0) return;

      try {
        const formData = new FormData();
        formData.append('enabled', store.selectedWorkflow.enabled.toString());

        // Use workflow ID for status update
        const workflowId = store.selectedWorkflow.id;
        const response = await POST(`${props.projectLink}/workflows/${workflowId}/status`, {
          data: formData,
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error('Failed to update workflow status:', errorText);
          showErrorToast(`${props.locale.failedToUpdateWorkflowStatus}: ${response.status} ${response.statusText}`);
          // Revert the status change on error
          store.selectedWorkflow.enabled = !store.selectedWorkflow.enabled;
          return;
        }

        const result = await response.json();
        if (result.success) {
          // Update workflow in the list
          const existingIndex = store.workflowEvents.findIndex((e) => e.event_id === store.selectedWorkflow.event_id);
          if (existingIndex >= 0) {
            store.workflowEvents[existingIndex].enabled = store.selectedWorkflow.enabled;
          }
        } else {
          // Revert the status change on failure
          store.selectedWorkflow.enabled = !store.selectedWorkflow.enabled;
          showErrorToast(`${props.locale.failedToUpdateWorkflowStatus}: Unexpected error`);
        }
      } catch (error) {
        console.error('Failed to update workflow status:', error);
        // Revert the status change on error
        store.selectedWorkflow.enabled = !store.selectedWorkflow.enabled;
        showErrorToast(`${props.locale.failedToUpdateWorkflowStatus}: ${error.message}`);
      }
    },

    async deleteWorkflow() {
      if (!store.selectedWorkflow || store.selectedWorkflow.id === 0) return;

      try {
        // Use workflow ID for deletion
        const workflowId = store.selectedWorkflow.id;
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
          const existingIndex = store.workflowEvents.findIndex((e) => e.event_id === store.selectedWorkflow.event_id);
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
