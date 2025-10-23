import {reactive} from 'vue';
import {GET, POST} from '../../modules/fetch.ts';
import {showInfoToast, showErrorToast} from '../../modules/toast.ts';

type WorkflowFiltersState = {
  issue_type: string;
  column: string;
  labels: string[];
};

type WorkflowIssueStateAction = '' | 'close' | 'reopen';

type WorkflowActionsState = {
  column: string;
  add_labels: string[];
  remove_labels: string[];
  issueState: WorkflowIssueStateAction;
};

type WorkflowDraftState = {
  filters: WorkflowFiltersState;
  actions: WorkflowActionsState;
};

const createDefaultFilters = (): WorkflowFiltersState => ({issue_type: '', column: '', labels: []});
const createDefaultActions = (): WorkflowActionsState => ({column: '', add_labels: [], remove_labels: [], issueState: ''});

const cloneFilters = (filters: WorkflowFiltersState): WorkflowFiltersState => ({
  issue_type: filters.issue_type,
  column: filters.column,
  labels: Array.from(filters.labels),
});

const cloneActions = (actions: WorkflowActionsState): WorkflowActionsState => ({
  column: actions.column,
  add_labels: Array.from(actions.add_labels),
  remove_labels: Array.from(actions.remove_labels),
  issueState: actions.issueState,
});

export function createWorkflowStore(props: {projectLink: string, eventID: string}) {
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

    updateDraft(eventId: string, filters: WorkflowFiltersState, actions: WorkflowActionsState) {
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
        console.log('[WorkflowStore] Loaded columns:', store.projectColumns);
        if (store.projectColumns.length > 0) {
          console.log('[WorkflowStore] First column.id type:', typeof store.projectColumns[0].id, 'value:', store.projectColumns[0].id);
        }
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
        console.log('[WorkflowStore] loadWorkflowData - eventId:', eventId);
        console.log('[WorkflowStore] loadWorkflowData - found workflow:', workflow);

          // Load existing configuration from the workflow data
          // Convert backend filter format to frontend format
        const frontendFilters = {issue_type: '', column: '', labels: []};
         // Convert backend action format to frontend format
        const frontendActions: WorkflowActionsState = {column: '', add_labels: [], remove_labels: [], issueState: ''};

        if (workflow?.filters && Array.isArray(workflow.filters)) {
          for (const filter of workflow.filters) {
            if (filter.type === 'issue_type') {
              frontendFilters.issue_type = filter.value;
            } else if (filter.type === 'column') {
              frontendFilters.column = filter.value;
            } else if (filter.type === 'labels') {
              frontendFilters.labels.push(filter.value);
            }
          }

          if (workflow.actions && Array.isArray(workflow.actions)) {
            for (const action of workflow.actions) {
              if (action.type === 'column') {
                // Backend returns string, keep as string to match column.id type
                frontendActions.column = action.value;
              } else if (action.type === 'add_labels') {
                // Backend returns string, keep as string to match label.id type
                frontendActions.add_labels.push(action.value);
              } else if (action.type === 'remove_labels') {
                // Backend returns string, keep as string to match label.id type
                frontendActions.remove_labels.push(action.value);
              } else if (action.type === 'close') {
                if (action.value === 'reopen' || action.value === 'false') {
                  frontendActions.issueState = 'reopen';
                } else if (action.value === 'true' || action.value === 'close') {
                  frontendActions.issueState = 'close';
                }
              }
            }
          }
        } else if (workflow?.actions && Array.isArray(workflow.actions)) {
          for (const action of workflow.actions) {
            if (action.type === 'column') {
              // Backend returns string, keep as string to match column.id type
              frontendActions.column = action.value;
            } else if (action.type === 'add_labels') {
              // Backend returns string, keep as string to match label.id type
              frontendActions.add_labels.push(action.value);
            } else if (action.type === 'remove_labels') {
              // Backend returns string, keep as string to match label.id type
              frontendActions.remove_labels.push(action.value);
            } else if (action.type === 'close') {
              if (action.value === 'reopen' || action.value === 'false') {
                frontendActions.issueState = 'reopen';
              } else if (action.value === 'true' || action.value === 'close') {
                frontendActions.issueState = 'close';
              }
            }
          }
        }

        store.workflowFilters = frontendFilters;
        store.workflowActions = frontendActions;
        store.updateDraft(eventId, frontendFilters, frontendActions);
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

      const currentEventId = store.selectedWorkflow?.event_id || store.selectedWorkflow?.base_event_type;
      if (currentEventId) {
        store.updateDraft(currentEventId, store.workflowFilters, store.workflowActions);
      }
    },

    async saveWorkflow() {
      if (!store.selectedWorkflow) return;

      store.saving = true;
      try {
        // For new workflows, use the base event type
        const eventId = store.selectedWorkflow.base_event_type || store.selectedWorkflow.event_id;
        const previousDraftKey = store.selectedWorkflow.event_id || store.selectedWorkflow.base_event_type;

        // Convert frontend data format to backend JSON format
        const postData = {
          event_id: eventId,
          filters: store.workflowFilters,
          actions: store.workflowActions,
        };

        // Send workflow data
        console.info('Sending workflow data:', postData);

        const response = await POST(`${props.projectLink}/workflows/${eventId}`, {
          data: postData,
          headers: {
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error('Response error:', errorText);
          showErrorToast(`Failed to save workflow: ${response.status} ${response.statusText}\n${errorText}`);
          return;
        }

        const result = await response.json();
        console.log('Response result:', result);
        if (result.success && result.workflow) {
          // Always reload the events list to get the updated structure
          // This ensures we have both the base event and the new filtered event
          const eventKey = typeof store.selectedWorkflow.event_id === 'string' ? store.selectedWorkflow.event_id : '';
          const wasNewWorkflow = store.selectedWorkflow.id === 0 ||
                                 eventKey.startsWith('new-') ||
                                 eventKey.startsWith('clone-');

          if (wasNewWorkflow && previousDraftKey) {
            store.clearDraft(previousDraftKey);
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

          // Convert backend data to frontend format and update form
          // Use the selectedWorkflow which now points to the reloaded workflow with complete data
          const frontendFilters = {issue_type: '', column: '', labels: []};
          const frontendActions: WorkflowActionsState = {column: '', add_labels: [], remove_labels: [], issueState: ''};

          if (store.selectedWorkflow.filters && Array.isArray(store.selectedWorkflow.filters)) {
            for (const filter of store.selectedWorkflow.filters) {
              if (filter.type === 'issue_type') {
                frontendFilters.issue_type = filter.value;
              } else if (filter.type === 'column') {
                frontendFilters.column = filter.value;
              } else if (filter.type === 'labels') {
                frontendFilters.labels.push(filter.value);
              }
            }
          }

          if (store.selectedWorkflow.actions && Array.isArray(store.selectedWorkflow.actions)) {
            for (const action of store.selectedWorkflow.actions) {
              if (action.type === 'column') {
                frontendActions.column = action.value;
              } else if (action.type === 'add_labels') {
                frontendActions.add_labels.push(action.value);
              } else if (action.type === 'remove_labels') {
                frontendActions.remove_labels.push(action.value);
              } else if (action.type === 'close') {
                if (action.value === 'reopen' || action.value === 'false') {
                  frontendActions.issueState = 'reopen';
                } else if (action.value === 'true' || action.value === 'close') {
                  frontendActions.issueState = 'close';
                }
              }
            }
          }

          store.workflowFilters = frontendFilters;
          store.workflowActions = frontendActions;
          if (store.selectedWorkflow?.event_id) {
            store.updateDraft(store.selectedWorkflow.event_id, frontendFilters, frontendActions);
          }

          // Update URL to use the new workflow ID
          if (wasNewWorkflow) {
            const newUrl = `${props.projectLink}/workflows/${store.selectedWorkflow.event_id}`;
            window.history.replaceState({eventId: store.selectedWorkflow.event_id}, '', newUrl);
          }

          showInfoToast('Workflow saved successfully!');
        } else {
          console.error('Unexpected response format:', result);
          showErrorToast('Failed to save workflow: Unexpected response format');
        }
      } catch (error) {
        console.error('Error saving workflow:', error);
        showErrorToast(`Error saving workflow: ${error.message}`);
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
          showErrorToast(`Failed to update workflow status: ${response.status} ${response.statusText}`);
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
          showErrorToast('Failed to update workflow status');
        }
      } catch (error) {
        console.error('Error updating workflow status:', error);
        // Revert the status change on error
        store.selectedWorkflow.enabled = !store.selectedWorkflow.enabled;
        showErrorToast(`Error updating workflow status: ${error.message}`);
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
          showErrorToast(`Failed to delete workflow: ${response.status} ${response.statusText}`);
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
          showErrorToast('Failed to delete workflow');
        }
      } catch (error) {
        console.error('Error deleting workflow:', error);
        showErrorToast(`Error deleting workflow: ${error.message}`);
      }
    },

  });
  return store;
}
