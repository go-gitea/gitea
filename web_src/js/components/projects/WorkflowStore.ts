import {reactive} from 'vue';
import {GET, POST} from '../../modules/fetch.ts';

export function createWorkflowStore(props: { projectLink: string, eventID: string}) {
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

    workflowFilters: {
      issue_type: '', // 'issue', 'pull_request', or ''
    },

    workflowActions: {
      column: '', // column ID to move to
      labels: [], // selected label IDs
      closeIssue: false,
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

        // Find the workflow from existing workflowEvents
        const workflow = store.workflowEvents.find((e) => e.event_id === eventId);
        if (workflow && workflow.filters && workflow.actions) {
          // Load existing configuration from the workflow data
          // Convert backend filter format to frontend format
          const frontendFilters = {issue_type: ''};
          if (workflow.filters && Array.isArray(workflow.filters)) {
            for (const filter of workflow.filters) {
              if (filter.type === 'issue_type') {
                frontendFilters.issue_type = filter.value;
              }
            }
          }

          // Convert backend action format to frontend format
          const frontendActions = {column: '', add_labels: [], closeIssue: false};
          if (workflow.actions && Array.isArray(workflow.actions)) {
            for (const action of workflow.actions) {
              if (action.action_type === 'column') {
                frontendActions.column = action.action_value;
              } else if (action.action_type === 'add_labels') {
                frontendActions.labels.push(action.action_value);
              } else if (action.action_type === 'close') {
                frontendActions.closeIssue = action.action_value === 'true';
              }
            }
          }

          store.workflowFilters = frontendFilters;
          store.workflowActions = frontendActions;
        } else {
          // Reset to defaults for new workflow
          store.resetWorkflowData();
        }
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
      store.workflowFilters = {issue_type: ''};
      store.workflowActions = {column: '', add_labels: [], closeIssue: false};
    },

    async saveWorkflow() {
      if (!store.selectedWorkflow) return;

      store.saving = true;
      try {
        // For new workflows, use the base event type
        const eventId = store.selectedWorkflow.base_event_type || store.selectedWorkflow.event_id;

        // Convert frontend data format to backend form format
        const formData = new FormData();
        formData.append('event_id', eventId);

        // Add filters as form fields
        for (const [key, value] of Object.entries(store.workflowFilters)) {
          if (value !== '') {
            formData.append(`filters[${key}]`, value);
          }
        }

        // Add actions as form fields
        for (const [key, value] of Object.entries(store.workflowActions)) {
          if (key === 'labels' && Array.isArray(value)) {
            // Handle label array
            for (const labelId of value) {
              if (labelId !== '') {
                formData.append(`actions[labels][]`, labelId);
              }
            }
          } else if (key === 'closeIssue') {
            // Handle boolean
            formData.append(`actions[${key}]`, value.toString());
          } else if (value !== '') {
            // Handle string fields
            formData.append(`actions[${key}]`, value);
          }
        }

        console.log('Saving workflow with FormData');
        console.log('URL:', `${props.projectLink}/workflows/${eventId}`);
        // Log form data entries
        for (const [key, value] of formData.entries()) {
          console.log(`${key}: ${value}`);
        }

        const response = await POST(`${props.projectLink}/workflows/${eventId}`, {
          data: formData,
        });

        console.log('Response status:', response.status);
        console.log('Response headers:', response.headers);

        if (!response.ok) {
          const errorText = await response.text();
          console.error('Response error:', errorText);
          alert(`Failed to save workflow: ${response.status} ${response.statusText}\n${errorText}`);
          return;
        }

        const result = await response.json();
        console.log('Response result:', result);
        if (result.success && result.workflow) {
          // Always reload the events list to get the updated structure
          // This ensures we have both the base event and the new filtered event
          const wasNewWorkflow = store.selectedWorkflow.id === 0 || 
                                 store.selectedWorkflow.event_id.startsWith('new-') || 
                                 store.selectedWorkflow.event_id.startsWith('clone-');

          // Reload events from server to get the correct event structure
          await store.loadEvents();

          // Update selected workflow and selectedItem
          store.selectedWorkflow = result.workflow;
          store.selectedItem = result.workflow.event_id;

          // Update URL to use the new workflow ID
          if (wasNewWorkflow) {
            const newUrl = `${props.projectLink}/workflows/${result.workflow.event_id}`;
            window.history.replaceState({eventId: result.workflow.event_id}, '', newUrl);
          }

          alert('Workflow saved successfully!');
        } else {
          console.error('Unexpected response format:', result);
          alert('Failed to save workflow: Unexpected response format');
        }
      } catch (error) {
        console.error('Error saving workflow:', error);
        alert(`Error saving workflow: ${error.message}`);
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
          alert(`Failed to update workflow status: ${response.status} ${response.statusText}`);
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
          console.log(`Workflow status updated to: ${store.selectedWorkflow.enabled ? 'enabled' : 'disabled'}`);
        } else {
          // Revert the status change on failure
          store.selectedWorkflow.enabled = !store.selectedWorkflow.enabled;
          alert('Failed to update workflow status');
        }
      } catch (error) {
        console.error('Error updating workflow status:', error);
        // Revert the status change on error
        store.selectedWorkflow.enabled = !store.selectedWorkflow.enabled;
        alert(`Error updating workflow status: ${error.message}`);
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
          alert(`Failed to delete workflow: ${response.status} ${response.statusText}`);
          return;
        }

        const result = await response.json();
        if (result.success) {
          // Remove workflow from the list
          const existingIndex = store.workflowEvents.findIndex((e) => e.event_id === store.selectedWorkflow.event_id);
          if (existingIndex >= 0) {
            store.workflowEvents.splice(existingIndex, 1);
          }
          console.log('Workflow deleted successfully');
        } else {
          alert('Failed to delete workflow');
        }
      } catch (error) {
        console.error('Error deleting workflow:', error);
        alert(`Error deleting workflow: ${error.message}`);
      }
    },

  });
  return store;
}
