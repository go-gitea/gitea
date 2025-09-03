import {reactive} from 'vue';
import {GET, POST} from '../../modules/fetch.ts';

export function createWorkflowStore(props: { projectLink: string, eventID: string}) {
  const store = reactive({
    workflowEvents: [],
    selectedItem: props.eventID,
    selectedWorkflow: null,
    projectColumns: [],
    saving: false,

    workflowFilters: {
      scope: '', // 'issue', 'pull_request', or ''
    },

    workflowActions: {
      column: '', // column ID to move to
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
      // Load project columns for the dropdown
      await store.loadProjectColumns();

      // Find the workflow from existing workflowEvents
      const workflow = store.workflowEvents.find((e) => e.event_id === eventId);
      if (workflow && workflow.filters && workflow.actions) {
        // Load existing configuration from the workflow data
        store.workflowFilters = workflow.filters || {scope: ''};
        store.workflowActions = workflow.actions || {column: '', closeIssue: false};
      } else {
        // Reset to defaults for new workflow
        store.resetWorkflowData();
      }
    },

    resetWorkflowData() {
      store.workflowFilters = {scope: ''};
      store.workflowActions = {column: '', closeIssue: false};
    },

    async saveWorkflow() {
      if (!store.selectedWorkflow) return;

      store.saving = true;
      try {
        const workflowData = {
          event_id: store.selectedWorkflow.event_id,
          filters: store.workflowFilters,
          actions: store.workflowActions,
        };

        const response = await POST(`${props.projectLink}/workflows/${store.selectedWorkflow.event_id}`, {
          data: workflowData,
        });

        if (!response.ok) {
          throw new Error('Failed to save workflow');
        }
      } catch (error) {
        console.error('Error saving workflow:', error);
      } finally {
        store.saving = false;
      }
    },

  });
  return store;
}
