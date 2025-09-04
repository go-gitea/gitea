<script lang="ts" setup>
import {onMounted, useTemplateRef, computed, ref} from 'vue';
import {createWorkflowStore} from './WorkflowStore.ts';
import {svg} from '../../svg.ts';

const elRoot = useTemplateRef('elRoot');

const props = defineProps({
  projectLink: {type: String, required: true},
  eventID: {type: String, required: true},
});

const store = createWorkflowStore(props);

const editMode = ref(false);

const toggleEditMode = () => {
  editMode.value = !editMode.value;
};

const toggleWorkflowStatus = async () => {
  if (store.selectedWorkflow) {
    // Toggle the enabled status
    store.selectedWorkflow.enabled = !store.selectedWorkflow.enabled;
    await store.saveWorkflowStatus();
  }
};

const deleteWorkflow = async () => {
  if (!store.selectedWorkflow || !confirm('Are you sure you want to delete this workflow?')) {
    return;
  }

  const currentBaseEventType = store.selectedWorkflow.base_event_type;
  const currentCapabilities = store.selectedWorkflow.capabilities;
  const currentDisplayName = store.selectedWorkflow.display_name.split(' (')[0]; // Remove filter suffix

  await store.deleteWorkflow();

  // Refresh workflow list
  store.workflowEvents = await store.loadEvents();

  // Find workflows for the same base event type
  const sameEventWorkflows = store.workflowEvents.filter(w =>
    w.base_event_type === currentBaseEventType ||
    w.workflow_event === currentBaseEventType
  );

  if (sameEventWorkflows.length === 0) {
    // No workflows left for this event type, create an empty one
    createNewWorkflow(currentBaseEventType, currentCapabilities, currentDisplayName);
    // URL already updated in createNewWorkflow
  } else {
    // Select the first remaining workflow of the same type
    selectWorkflowItem(sameEventWorkflows[0]);
    // URL already updated in selectWorkflowItem
  }

  editMode.value = false;
};

const selectWorkflowEvent = (event) => {
  // Toggle selection - if already selected, deselect
  if (store.selectedItem === event.event_id) {
    store.selectedItem = null;
    store.selectedWorkflow = null;
    return;
  }

  store.selectedItem = event.event_id;
  store.selectedWorkflow = event;
  store.loadWorkflowData(event.event_id);

  // Update URL without page reload
  const newUrl = `${props.projectLink}/workflows/${event.event_id}`;
  window.history.pushState({eventId: event.event_id}, '', newUrl);
};

const saveWorkflow = async () => {
  await store.saveWorkflow();
  // After saving, refresh the list to show the new workflow
  store.workflowEvents = await store.loadEvents();
};

const isWorkflowConfigured = (event) => {
  // Check if the event_id is a number (saved workflow ID) vs UUID (unconfigured)
  return !Number.isNaN(parseInt(event.event_id));
};

// Get flat list of all workflows - directly use backend data
const workflowList = computed(() => {
  return store.workflowEvents.map((workflow) => ({
    ...workflow,
    isConfigured: isWorkflowConfigured(workflow),
    base_event_type: workflow.event_id,
  }));
});

const createNewWorkflow = (baseEventType, capabilities, displayName) => {
  const tempId = `new-${baseEventType}-${Date.now()}`;
  const newWorkflow = {
    id: 0,
    event_id: tempId,
    display_name: displayName,
    capabilities,
    filters: [],
    actions: [],
    filter_summary: '',
    base_event_type: baseEventType,
    enabled: true,
  };

  store.selectedWorkflow = newWorkflow;
  store.selectedItem = tempId;
  store.resetWorkflowData();
  editMode.value = true; // Auto-enter edit mode for new workflows
};

const cloneWorkflow = (sourceWorkflow) => {
  const tempId = `clone-${sourceWorkflow.base_event_type || sourceWorkflow.workflow_event}-${Date.now()}`;
  const clonedWorkflow = {
    id: 0,
    event_id: tempId,
    display_name: sourceWorkflow.display_name.split(' (')[0], // Remove filter suffix
    capabilities: sourceWorkflow.capabilities,
    filters: Array.from(sourceWorkflow.filters || []),
    actions: Array.from(sourceWorkflow.actions || []),
    filter_summary: '',
    base_event_type: sourceWorkflow.base_event_type || sourceWorkflow.workflow_event,
    enabled: true,
  };

  store.selectedWorkflow = clonedWorkflow;
  store.selectedItem = tempId;

  // Load the source workflow's data into the form
  store.loadWorkflowData(sourceWorkflow.event_id);
  editMode.value = true; // Auto-enter edit mode for cloned workflows
  
  // Update URL for cloned workflow
  const newUrl = `${props.projectLink}/workflows/${tempId}`;
  window.history.pushState({eventId: tempId}, '', newUrl);
};

const selectWorkflowItem = (item) => {
  editMode.value = false; // Reset edit mode when switching workflows
  if (item.isConfigured) {
    // This is a configured workflow, select it
    selectWorkflowEvent(item);
  } else {
    // This is an unconfigured event, create new workflow
    createNewWorkflow(item.base_event_type, item.capabilities, item.display_name);
    // Update URL for new workflow
    const newUrl = `${props.projectLink}/workflows/${item.base_event_type}`;
    window.history.pushState({eventId: item.base_event_type}, '', newUrl);
  }
};

const hasAvailableFilters = computed(() => {
  return store.selectedWorkflow?.capabilities?.available_filters?.length > 0;
});

const hasFilter = (filterType) => {
  return store.selectedWorkflow?.capabilities?.available_filters?.includes(filterType);
};

const hasAction = (actionType) => {
  return store.selectedWorkflow?.capabilities?.available_actions?.includes(actionType);
};

const _getActionsSummary = (workflow) => {
  if (!workflow.actions || workflow.actions.length === 0) {
    return '';
  }

  const actions = [];
  for (const action of workflow.actions) {
    if (action.action_type === 'column') {
      const column = store.projectColumns.find((c) => c.id === action.action_value);
      if (column) {
        actions.push(`Move to "${column.title}"`);
      }
    } else if (action.action_type === 'add_labels') {
      const label = store.projectLabels.find((l) => l.id === action.action_value);
      if (label) {
        actions.push(`Add label "${label.name}"`);
      }
    } else if (action.action_type === 'remove_labels') {
      const label = store.projectLabels.find((l) => l.id === action.action_value);
      if (label) {
        actions.push(`Remove label "${label.name}"`);
      }
    } else if (action.action_type === 'close') {
      actions.push('Close issue');
    }
  }

  return actions.join(', ');
};

onMounted(async () => {
  // Load all necessary data
  store.workflowEvents = await store.loadEvents();
  await store.loadProjectColumns();
  await store.loadProjectLabels();

  // Auto-select logic
  if (props.eventID) {
    // If eventID is provided in URL, try to find and select it
    const selectedEvent = store.workflowEvents.find((e) => e.event_id === props.eventID);
    if (selectedEvent) {
      // Found existing configured workflow
      store.selectedItem = props.eventID;
      store.selectedWorkflow = selectedEvent;
      await store.loadWorkflowData(props.eventID);
      editMode.value = false;
    } else {
      // Check if eventID matches a base event type (unconfigured workflow)
      const items = workflowList.value;
      const matchingUnconfigured = items.find((item) => 
        !item.isConfigured && (item.base_event_type === props.eventID || item.event_id === props.eventID)
      );
      if (matchingUnconfigured) {
        // Create new workflow for this base event type
        createNewWorkflow(matchingUnconfigured.base_event_type, matchingUnconfigured.capabilities, matchingUnconfigured.display_name);
      } else {
        // Fallback: select first available item
        if (items.length > 0) {
          const firstConfigured = items.find((item) => item.isConfigured);
          if (firstConfigured) {
            selectWorkflowItem(firstConfigured);
          } else {
            selectWorkflowItem(items[0]);
          }
        }
      }
    }
  } else {
    // Auto-select first configured workflow, or first item if none configured
    const items = workflowList.value;
    if (items.length > 0) {
      // Find first configured workflow
      const firstConfigured = items.find((item) => item.isConfigured);

      if (firstConfigured) {
        // Select first configured workflow
        selectWorkflowItem(firstConfigured);
      } else {
        // No configured workflows, select first item
        selectWorkflowItem(items[0]);
      }
    }
  }

  elRoot.value.closest('.is-loading')?.classList?.remove('is-loading');

  window.addEventListener('popstate', (e) => {
    if (e.state?.eventId) {
      // Handle browser back/forward navigation
      const event = store.workflowEvents.find((ev) => ev.event_id === e.state.eventId);
      if (event) {
        selectWorkflowEvent(event);
      } else {
        // Check if it's a base event type
        const items = workflowList.value;
        const matchingUnconfigured = items.find((item) => 
          !item.isConfigured && (item.base_event_type === e.state.eventId || item.event_id === e.state.eventId)
        );
        if (matchingUnconfigured) {
          createNewWorkflow(matchingUnconfigured.base_event_type, matchingUnconfigured.capabilities, matchingUnconfigured.display_name);
        }
      }
    }
  });
});
</script>

<template>
  <div ref="elRoot" class="workflow-container">
    <!-- Left Sidebar - Workflow List -->
    <div class="workflow-sidebar">
      <div class="sidebar-header">
        <h3>Project Workflows</h3>
      </div>

      <div class="sidebar-content">
        <!-- Flat Workflow List -->
        <div class="workflow-items">
          <div
            v-for="item in workflowList"
            :key="item.event_id"
            class="workflow-item"
            :class="{ active: store.selectedItem === item.event_id }"
            @click="selectWorkflowItem(item)"
          >
            <div class="workflow-content">
              <div class="workflow-info">
                <span class="status-indicator">
                  <span
                    v-html="svg('octicon-dot-fill')"
                    :class="item.isConfigured ? 'status-active' : 'status-inactive'"
                  />
                </span>
                <div class="workflow-title">{{ item.display_name }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Right Main Content - Editor -->
    <div class="workflow-main">
      <!-- Default State -->
      <div v-if="!store.selectedWorkflow" class="workflow-placeholder">
        <div class="placeholder-content">
          <div class="placeholder-icon">
            <i class="huge settings icon"/>
          </div>
          <h3>Select a workflow to configure</h3>
          <p>Choose an event from the left sidebar to create or configure workflows.</p>
        </div>
      </div>

      <!-- Workflow Editor -->
      <div v-else class="workflow-editor">
        <div class="editor-header">
          <div class="editor-title">
            <h2>
              <i class="settings icon"/>
              {{ store.selectedWorkflow.display_name }}
              <span v-if="store.selectedWorkflow.id > 0 && !editMode"
                    class="workflow-status"
                    :class="store.selectedWorkflow.enabled ? 'status-enabled' : 'status-disabled'">
                {{ store.selectedWorkflow.enabled ? 'Enabled' : 'Disabled' }}
              </span>
            </h2>
            <p v-if="editMode">Configure automated actions for this workflow</p>
            <p v-else>View workflow configuration</p>
          </div>
          <div class="editor-actions-header">
            <!-- Edit/Cancel Button -->
            <button
              class="ui button"
              :class="editMode ? 'basic' : 'primary'"
              @click="toggleEditMode"
            >
              <i :class="editMode ? 'times icon' : 'edit icon'"/>
              {{ editMode ? 'Cancel' : 'Edit' }}
            </button>

            <!-- Enable/Disable Button (only for configured workflows) -->
            <button
              v-if="store.selectedWorkflow && store.selectedWorkflow.id > 0 && !editMode"
              class="ui button"
              :class="store.selectedWorkflow.enabled ? 'red basic' : 'green'"
              @click="toggleWorkflowStatus"
              :title="store.selectedWorkflow.enabled ? 'Disable workflow' : 'Enable workflow'"
            >
              <i :class="store.selectedWorkflow.enabled ? 'pause icon' : 'play icon'"/>
              {{ store.selectedWorkflow.enabled ? 'Disable' : 'Enable' }}
            </button>

            <!-- Clone Button (only for configured workflows) -->
            <button
              v-if="store.selectedWorkflow && store.selectedWorkflow.id > 0 && !editMode"
              class="ui basic button"
              @click="cloneWorkflow(store.selectedWorkflow)"
              title="Clone this workflow"
            >
              <i class="copy icon"/>
              Clone
            </button>
          </div>
        </div>

        <div class="editor-content">
          <div class="ui form" :class="{ 'readonly': !editMode }">
            <div class="field">
              <label>When</label>
              <div class="ui segment">
                <div class="description">
                  This workflow will run when: <strong>{{ store.selectedWorkflow.display_name }}</strong>
                </div>
              </div>
            </div>

            <!-- Filters Section -->
            <div class="field" v-if="hasAvailableFilters">
              <label>Filters</label>
              <div class="ui segment">
                <div class="field" v-if="hasFilter('issue_type')">
                  <label>Apply to</label>
                  <select
                    v-if="editMode"
                    class="ui dropdown"
                    v-model="store.workflowFilters.issue_type"
                  >
                    <option value="">Issues And Pull Requests</option>
                    <option value="issue">Issues</option>
                    <option value="pull_request">Pull requests</option>
                  </select>
                  <div v-else class="readonly-value">
                    {{ store.workflowFilters.issue_type === 'issue' ? 'Issues' :
                       store.workflowFilters.issue_type === 'pull_request' ? 'Pull requests' :
                       'Issues And Pull Requests' }}
                  </div>
                </div>
              </div>
            </div>

            <!-- Actions Section -->
            <div class="field">
              <label>Actions</label>
              <div class="ui segment">
                <div class="field" v-if="hasAction('column')">
                  <label>Move to column</label>
                  <select
                    v-if="editMode"
                    class="ui dropdown"
                    v-model="store.workflowActions.column"
                  >
                    <option value="">Select column...</option>
                    <option v-for="column in store.projectColumns" :key="column.id" :value="column.id">
                      {{ column.title }}
                    </option>
                  </select>
                  <div v-else class="readonly-value">
                    {{ store.projectColumns.find(c => c.id === store.workflowActions.column)?.title || 'None' }}
                  </div>
                </div>

                <div class="field" v-if="hasAction('label')">
                  <label>Add labels</label>
                  <select
                    v-if="editMode"
                    class="ui multiple dropdown"
                    v-model="store.workflowActions.labels"
                  >
                    <option value="">Select labels...</option>
                    <option v-for="label in store.projectLabels" :key="label.id" :value="label.id">
                      {{ label.name }}
                    </option>
                  </select>
                  <div v-else class="readonly-value">
                    {{ store.workflowActions.labels?.map(id =>
                       store.projectLabels.find(l => l.id === id)?.name).join(', ') || 'None' }}
                  </div>
                </div>

                <div class="field" v-if="hasAction('close')">
                  <div v-if="editMode" class="ui checkbox">
                    <input type="checkbox" v-model="store.workflowActions.closeIssue" id="close-issue">
                    <label for="close-issue">Close issue</label>
                  </div>
                  <div v-else class="readonly-value">
                    <label>Close issue</label>
                    <div>{{ store.workflowActions.closeIssue ? 'Yes' : 'No' }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Fixed bottom actions (only show in edit mode) -->
        <div v-if="editMode" class="editor-actions">
          <button class="ui primary button" @click="saveWorkflow" :class="{ loading: store.saving }">
            <i class="save icon"/>
            Save Workflow
          </button>
          <button
            v-if="store.selectedWorkflow && store.selectedWorkflow.id > 0"
            class="ui red button"
            @click="deleteWorkflow"
          >
            <i class="trash icon"/>
            Delete
          </button>
        </div>
      </div>
    </div>

  </div>
</template>

<style scoped>
/* Main Layout */
.workflow-container {
  display: flex;
  width: 100%;
  height: calc(100vh - 200px);
  min-height: 600px;
  border: 1px solid #e1e4e8;
  border-radius: 8px;
  overflow: hidden;
  background: white;
}

.workflow-sidebar {
  width: 350px;
  flex-shrink: 0;
  background: #f6f8fa;
  border-right: 1px solid #e1e4e8;
  display: flex;
  flex-direction: column;
}

.workflow-main {
  flex: 1;
  background: white;
  display: flex;
  flex-direction: column;
}

/* Sidebar */
.sidebar-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid #e1e4e8;
  background: #f6f8fa;
}

.sidebar-header h3 {
  margin: 0;
  color: #24292e;
  font-size: 1.1rem;
  font-weight: 600;
}

.sidebar-content {
  flex: 1;
  padding: 1rem;
  overflow-y: auto;
}

/* Workflow Items */
.workflow-items {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.workflow-item {
  padding: 0.75rem 1rem;
  cursor: pointer;
  transition: all 0.2s ease;
  border-radius: 6px;
  margin-bottom: 0.25rem;
}

.workflow-item:hover {
  background: #f6f8fa;
}

.workflow-item.active {
  background: #f1f8ff;
  border-left: 3px solid #0366d6;
}

.workflow-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
}

.workflow-info {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.workflow-title {
  font-weight: 500;
  color: #24292e;
  font-size: 0.9rem;
  line-height: 1.3;
}

.status-indicator .status-active {
  color: #28a745;
  font-size: 0.75rem;
}

.status-indicator .status-inactive {
  color: #d1d5da;
  font-size: 0.75rem;
}

/* Main Content Area */
.workflow-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2rem;
}

.placeholder-content {
  text-align: center;
  max-width: 400px;
}

.placeholder-icon {
  margin-bottom: 1.5rem;
  color: #d1d5da;
}

.placeholder-content h3 {
  color: #24292e;
  margin-bottom: 0.5rem;
  font-weight: 600;
}

.placeholder-content p {
  color: #586069;
  margin-bottom: 2rem;
  line-height: 1.5;
}

/* Editor */
.workflow-editor {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 1.5rem;
  border-bottom: 1px solid #e1e4e8;
  background: #fafbfc;
}

.editor-title h2 {
  margin: 0 0 0.25rem 0;
  color: #24292e;
  font-size: 1.25rem;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.editor-title p {
  margin: 0;
  color: #586069;
  font-size: 0.9rem;
}

.editor-actions-header {
  flex-shrink: 0;
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.editor-content {
  flex: 1;
  padding: 1.5rem;
  overflow-y: auto;
}

.editor-content .field {
  margin-bottom: 1.5rem;
}

.editor-content .field label {
  font-weight: 600;
  color: #24292e;
  margin-bottom: 0.5rem;
  display: block;
}

.editor-content .ui.segment {
  background: #fafbfc;
  border: 1px solid #e1e4e8;
  padding: 1rem;
  margin-bottom: 0.5rem;
}

.editor-content .description {
  color: #586069;
  font-size: 0.9rem;
}

.editor-actions {
  display: flex;
  gap: 0.5rem;
  padding: 1.5rem;
  border-top: 1px solid #e1e4e8;
  background: white;
  flex-shrink: 0;
}

/* Responsive */
@media (max-width: 768px) {
  .workflow-container {
    flex-direction: column;
    height: auto;
  }

  .workflow-sidebar {
    width: 100%;
    max-height: 40vh;
    border-right: none;
    border-bottom: 1px solid #e1e4e8;
  }

  .editor-header {
    flex-direction: column;
    gap: 1rem;
    align-items: stretch;
  }

  .editor-content {
    padding: 1rem;
  }

  .editor-actions {
    flex-direction: column;
  }
}

@media (max-width: 480px) {
  .sidebar-header {
    flex-direction: column;
    gap: 0.5rem;
    align-items: stretch;
  }

  .workflow-item {
    padding: 0.75rem;
  }

  .editor-actions button {
    width: 100%;
  }
}

/* Workflow status styles */
.workflow-status {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  margin-left: 0.5rem;
}

.workflow-status.status-enabled {
  background: #d4edda;
  color: #155724;
  border: 1px solid #c3e6cb;
}

.workflow-status.status-disabled {
  background: #f8d7da;
  color: #721c24;
  border: 1px solid #f5c6cb;
}

/* Readonly form styles */
.ui.form.readonly {
  pointer-events: none;
}

.readonly-value {
  background: #f6f8fa;
  padding: 0.5rem;
  border: 1px solid #e1e4e8;
  border-radius: 4px;
  color: #24292e;
  font-weight: 500;
}

.readonly-value label {
  font-weight: 600;
  margin-bottom: 0.25rem;
  display: block;
}

.readonly-value div {
  color: #586069;
  font-weight: normal;
}
</style>
