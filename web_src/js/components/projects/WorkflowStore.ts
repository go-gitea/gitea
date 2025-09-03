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
    showCreateDialog: false, // For create workflow dialog
    selectedEventType: null, // For workflow creation

    workflowFilters: {
      scope: '', // 'issue', 'pull_request', or ''
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
      // Load project columns and labels for the dropdowns
      await store.loadProjectColumns();
      await store.loadProjectLabels();

      // Find the workflow from existing workflowEvents
      const workflow = store.workflowEvents.find((e) => e.event_id === eventId);
      if (workflow && workflow.filters && workflow.actions) {
        // Load existing configuration from the workflow data
        // Convert backend filter format to frontend format
        const frontendFilters = {scope: ''};
        if (workflow.filters && Array.isArray(workflow.filters)) {
          for (const filter of workflow.filters) {
            if (filter.type === 'scope') {
              frontendFilters.scope = filter.value;
            }
          }
        }

        // Convert backend action format to frontend format
        const frontendActions = {column: '', labels: [], closeIssue: false};
        if (workflow.actions && Array.isArray(workflow.actions)) {
          for (const action of workflow.actions) {
            if (action.action_type === 'column') {
              frontendActions.column = action.action_value;
            } else if (action.action_type === 'label') {
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
      store.workflowFilters = {scope: ''};
      store.workflowActions = {column: '', labels: [], closeIssue: false};
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
          // For new workflows, add to the store
          if (store.selectedWorkflow.id === 0 || store.selectedWorkflow.event_id.startsWith('new-')) {
            store.workflowEvents.push(result.workflow);

            // Update URL to use the new workflow ID
            const newUrl = `${props.projectLink}/workflows/${result.workflow.event_id}`;
            window.history.replaceState({eventId: result.workflow.event_id}, '', newUrl);
          } else {
            // Update existing workflow
            const existingIndex = store.workflowEvents.findIndex((e) => e.event_id === store.selectedWorkflow.event_id);
            if (existingIndex >= 0) {
              store.workflowEvents[existingIndex] = {
                ...store.workflowEvents[existingIndex],
                ...result.workflow,
              };
            }
          }

          // Update selected workflow and selectedItem
          store.selectedWorkflow = result.workflow;
          store.selectedItem = result.workflow.event_id;
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

  });
  return store;
}
