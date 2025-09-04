<script lang="ts" setup>
import {onMounted, onUnmounted, useTemplateRef, computed, ref, nextTick} from 'vue';
import {createWorkflowStore} from './WorkflowStore.ts';
import {svg} from '../../svg.ts';

const elRoot = useTemplateRef('elRoot');

const props = defineProps({
  projectLink: {type: String, required: true},
  eventID: {type: String, required: true},
});

const store = createWorkflowStore(props);

// Track edit state directly on workflow objects
const previousSelection = ref(null);

// Helper to check if current workflow is in edit mode
const isInEditMode = computed(() => {
  if (!store.selectedWorkflow) return false;

  // Unconfigured workflows (id === 0) are always in edit mode
  if (store.selectedWorkflow.id === 0) {
    return true;
  }

  // Configured workflows use the _isEditing flag
  return store.selectedWorkflow._isEditing || false;
});

// Helper to set edit mode for current workflow
const setEditMode = (enabled) => {
  if (store.selectedWorkflow) {
    if (enabled) {
      store.selectedWorkflow._isEditing = true;
    } else {
      delete store.selectedWorkflow._isEditing;
    }
  }
};

 // Store previous selection for cancel functionality

const toggleEditMode = () => {
  if (isInEditMode.value) {
    // Canceling edit mode
    if (previousSelection.value) {
      // If there was a previous selection, return to it
      if (store.selectedWorkflow && store.selectedWorkflow.id === 0) {
        // Remove temporary cloned workflow from list
        const tempIndex = store.workflowEvents.findIndex(w =>
          w.event_id === store.selectedWorkflow.event_id
        );
        if (tempIndex >= 0) {
          store.workflowEvents.splice(tempIndex, 1);
        }
      }

      // Restore previous selection
      store.selectedItem = previousSelection.value.selectedItem;
      store.selectedWorkflow = previousSelection.value.selectedWorkflow;
      if (previousSelection.value.selectedWorkflow) {
        store.loadWorkflowData(previousSelection.value.selectedWorkflow.event_id);
      }
      previousSelection.value = null;
    }
    setEditMode(false);
  } else {
    // Entering edit mode - store current selection
    previousSelection.value = {
      selectedItem: store.selectedItem,
      selectedWorkflow: store.selectedWorkflow ? {...store.selectedWorkflow} : null
    };
    setEditMode(true);
  }
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

  const currentBaseEventType = store.selectedWorkflow.base_event_type || store.selectedWorkflow.workflow_event || store.selectedWorkflow.event_id;
  const currentCapabilities = store.selectedWorkflow.capabilities;
  // Extract base name without any parenthetical descriptions
  const currentDisplayName = (store.selectedWorkflow.display_name || store.selectedWorkflow.workflow_event || store.selectedWorkflow.event_id)
    .replace(/\s*\([^)]*\)\s*/g, '');

  // If deleting a temporary workflow (clone/new), just remove from list
  if (store.selectedWorkflow.id === 0) {
    const tempIndex = store.workflowEvents.findIndex(w =>
      w.event_id === store.selectedWorkflow.event_id
    );
    if (tempIndex >= 0) {
      store.workflowEvents.splice(tempIndex, 1);
    }
  } else {
    // Delete from backend
    await store.deleteWorkflow();
    // Refresh workflow list
    store.workflowEvents = await store.loadEvents();
  }

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

  // Clear previous selection and exit edit mode
  previousSelection.value = null;
  setEditMode(false);
};

const selectWorkflowEvent = async (event) => {
  // Prevent rapid successive clicks
  if (store.loading) return;

  // Toggle selection - if already selected, deselect
  if (store.selectedItem === event.event_id) {
    store.selectedItem = null;
    store.selectedWorkflow = null;
    return;
  }

  try {
    store.selectedItem = event.event_id;
    store.selectedWorkflow = event;

    // Wait for DOM update before proceeding
    await nextTick();

    await store.loadWorkflowData(event.event_id);

    // Update URL without page reload
    const newUrl = `${props.projectLink}/workflows/${event.event_id}`;
    window.history.pushState({eventId: event.event_id}, '', newUrl);
  } catch (error) {
    console.error('Error selecting workflow event:', error);
    // Reset state on error
    store.selectedItem = null;
    store.selectedWorkflow = null;
  }
};

const saveWorkflow = async () => {
  await store.saveWorkflow();
  // The store.saveWorkflow already handles reloading events

  // Clear previous selection after successful save
  previousSelection.value = null;
  setEditMode(false);
};

const isWorkflowConfigured = (event) => {
  // Check if the event_id is a number (saved workflow ID) or if it has id > 0
  return !Number.isNaN(parseInt(event.event_id)) || (event.id !== undefined && event.id > 0);
};

// Generate filter description for display name
const getFilterDescription = (workflow) => {
  if (!workflow.filters || !Array.isArray(workflow.filters) || workflow.filters.length === 0) {
    return '';
  }

  const descriptions = [];
  for (const filter of workflow.filters) {
    if (filter.type === 'issue_type' && filter.value) {
      if (filter.value === 'issue') {
        descriptions.push('Issues');
      } else if (filter.value === 'pull_request') {
        descriptions.push('Pull Requests');
      }
    }
    // Add more filter types here as needed
  }

  return descriptions.length > 0 ? ` (${descriptions.join(', ')})` : '';
};

// Get display name with filters
const getWorkflowDisplayName = (workflow) => {
  const baseName = workflow.display_name || workflow.workflow_event || workflow.event_id;
  if (isWorkflowConfigured(workflow)) {
    return baseName + getFilterDescription(workflow);
  }
  return baseName;
};

// Get flat list of all workflows - use cached data to prevent frequent recomputation
const workflowList = computed(() => {
  // Use a stable reference to prevent unnecessary DOM updates
  const workflows = store.workflowEvents;
  if (!workflows || workflows.length === 0) {
    return [];
  }

  return workflows.map((workflow) => ({
    ...workflow,
    isConfigured: isWorkflowConfigured(workflow),
    base_event_type: workflow.base_event_type || workflow.workflow_event || workflow.event_id,
    display_name: getWorkflowDisplayName(workflow),
  }));
});

const createNewWorkflow = (baseEventType, capabilities, displayName) => {
  // Store current selection before creating new workflow
  if (!isInEditMode.value) {
    previousSelection.value = {
      selectedItem: store.selectedItem,
      selectedWorkflow: store.selectedWorkflow ? {...store.selectedWorkflow} : null
    };
  }

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
    workflow_event: baseEventType,
    enabled: true, // Ensure new workflows are enabled by default
  };

  store.selectedWorkflow = newWorkflow;
  // For unconfigured events, use the base event type as selected item for UI consistency
  store.selectedItem = baseEventType;
  store.resetWorkflowData();
  // Unconfigured workflows are always in edit mode by default
};

const cloneWorkflow = (sourceWorkflow) => {
  // Store current selection before cloning
  previousSelection.value = {
    selectedItem: store.selectedItem,
    selectedWorkflow: store.selectedWorkflow ? {...store.selectedWorkflow} : null
  };

  const tempId = `clone-${sourceWorkflow.base_event_type || sourceWorkflow.workflow_event}-${Date.now()}`;
  // Extract base name without filter descriptions
  const baseName = (sourceWorkflow.display_name || sourceWorkflow.workflow_event || sourceWorkflow.event_id)
    .replace(/\s*\([^)]*\)\s*/g, ''); // Remove any parenthetical descriptions

  const clonedWorkflow = {
    id: 0,
    event_id: tempId,
    display_name: `${baseName} (Copy)`, // Add copy suffix
    capabilities: sourceWorkflow.capabilities,
    filters: Array.from(sourceWorkflow.filters || []),
    actions: Array.from(sourceWorkflow.actions || []),
    filter_summary: '',
    base_event_type: sourceWorkflow.base_event_type || sourceWorkflow.workflow_event || sourceWorkflow.event_id,
    enabled: true,
  };

  // Find the position of source workflow and insert cloned workflow after it
  const sourceIndex = store.workflowEvents.findIndex(w => w.event_id === sourceWorkflow.event_id);
  if (sourceIndex >= 0) {
    store.workflowEvents.splice(sourceIndex + 1, 0, clonedWorkflow);
  } else {
    store.workflowEvents.push(clonedWorkflow);
  }

  // Select the cloned workflow
  store.selectedWorkflow = clonedWorkflow;
  store.selectedItem = tempId;

  // Load the source workflow's data into the form
  store.loadWorkflowData(sourceWorkflow.event_id);
  // Cloned workflows (id: 0) are always in edit mode by default

  // Update URL for cloned workflow
  const newUrl = `${props.projectLink}/workflows/${tempId}`;
  window.history.pushState({eventId: tempId}, '', newUrl);
};

// Add debounce mechanism
let selectTimeout = null;

const selectWorkflowItem = async (item) => {
  // Prevent rapid successive clicks with debounce
  if (store.loading || selectTimeout) return;

  selectTimeout = setTimeout(() => {
    selectTimeout = null;
  }, 300);

  previousSelection.value = null; // Clear previous selection when manually selecting
  // Don't reset edit mode when switching - each workflow keeps its own state

  // Wait for DOM update to prevent conflicts
  await nextTick();

  if (item.isConfigured) {
    // This is a configured workflow, select it
    await selectWorkflowEvent(item);
  } else {
    // This is an unconfigured event - check if we already have a workflow object for it
    const existingWorkflow = store.workflowEvents.find(w =>
      w.id === 0 &&
      (w.base_event_type === item.base_event_type || w.workflow_event === item.base_event_type)
    );

    if (existingWorkflow) {
      // We already have an unconfigured workflow for this event type, select it
      await selectWorkflowEvent(existingWorkflow);
    } else {
      // This is truly a new unconfigured event, create new workflow
      createNewWorkflow(item.base_event_type, item.capabilities, item.display_name);
    }

    // Update URL for workflow
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

const getStatusClass = (item) => {
  if (!item.isConfigured) {
    return 'status-inactive'; // Gray dot for unconfigured
  }

  // For configured workflows, check enabled status
  if (item.enabled === false) {
    return 'status-disabled'; // Red dot for disabled
  }

  return 'status-active'; // Green dot for enabled
};

const isItemSelected = (item) => {
  if (!store.selectedItem) return false;

  if (item.isConfigured || item.id === 0) {
    // For configured workflows or temporary workflows (clones/new), match by event_id
    return store.selectedItem === item.event_id;
  } else {
    // For unconfigured events, match by base_event_type
    return store.selectedItem === item.base_event_type;
  }
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

  // Add native event listener to prevent conflicts with Gitea
  await nextTick();
  const workflowItemsContainer = elRoot.value.querySelector('.workflow-items');
  if (workflowItemsContainer) {
    workflowClickHandler = (e) => {
      const workflowItem = e.target.closest('.workflow-item');
      if (workflowItem) {
        e.preventDefault();
        e.stopPropagation();
        const itemData = workflowItem.getAttribute('data-workflow-item');
        if (itemData) {
          try {
            const item = JSON.parse(itemData);
            selectWorkflowItem(item);
          } catch (error) {
            console.error('Error parsing workflow item data:', error);
          }
        }
      }
    };
    workflowItemsContainer.addEventListener('click', workflowClickHandler);
  }

  // Auto-select logic
  if (props.eventID) {
    // If eventID is provided in URL, try to find and select it
    const selectedEvent = store.workflowEvents.find((e) => e.event_id === props.eventID);
    if (selectedEvent) {
      // Found existing configured workflow
      store.selectedItem = props.eventID;
      store.selectedWorkflow = selectedEvent;
      await store.loadWorkflowData(props.eventID);
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

  window.addEventListener('popstate', popstateHandler);
});

// Define popstateHandler at component level
const popstateHandler = (e) => {
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
};

// Store reference to cleanup event listener
let workflowClickHandler = null;

onUnmounted(() => {
  // Clean up resources
  if (selectTimeout) {
    clearTimeout(selectTimeout);
    selectTimeout = null;
  }
  window.removeEventListener('popstate', popstateHandler);

  // Remove native click event listener
  const workflowItemsContainer = elRoot.value?.querySelector('.workflow-items');
  if (workflowItemsContainer && workflowClickHandler) {
    workflowItemsContainer.removeEventListener('click', workflowClickHandler);
  }
});
</script>

<template>
  <div ref="elRoot" class="workflow-container">
    <!-- Left Sidebar - Workflow List -->
    <div class="workflow-sidebar">
      <div class="sidebar-header">
        <h3>Default Workflows</h3>
      </div>

      <div class="sidebar-content">
        <!-- Flat Workflow List -->
        <div class="workflow-items">
          <div
            v-for="item in workflowList"
            :key="`workflow-${item.event_id}-${item.isConfigured ? 'configured' : 'unconfigured'}`"
            class="workflow-item"
            :class="{ active: isItemSelected(item) }"
            :data-workflow-item="JSON.stringify(item)"
          >
            <div class="workflow-content">
              <div class="workflow-info">
                <span class="status-indicator">
                  <span
                    v-html="svg('octicon-dot-fill')"
                    :class="getStatusClass(item)"
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
              <span v-if="store.selectedWorkflow.id > 0 && !isInEditMode"
                    class="workflow-status"
                    :class="store.selectedWorkflow.enabled ? 'status-enabled' : 'status-disabled'">
                {{ store.selectedWorkflow.enabled ? 'Enabled' : 'Disabled' }}
              </span>
            </h2>
            <p v-if="store.selectedWorkflow.id === 0">Configure automated actions for this workflow</p>
            <p v-else-if="isInEditMode">Configure automated actions for this workflow</p>
            <p v-else>View workflow configuration</p>
          </div>
          <div class="editor-actions-header">
            <!-- Edit/Cancel Button (only for configured workflows) -->
            <button
              v-if="store.selectedWorkflow && store.selectedWorkflow.id > 0"
              class="btn"
              :class="isInEditMode ? 'btn-outline-secondary' : 'btn-primary'"
              @click="toggleEditMode"
            >
              <i :class="isInEditMode ? 'times icon' : 'edit icon'"/>
              {{ isInEditMode ? 'Cancel' : 'Edit' }}
            </button>

            <!-- Enable/Disable Button (only for configured workflows) -->
            <button
              v-if="store.selectedWorkflow && store.selectedWorkflow.id > 0 && !isInEditMode"
              class="btn"
              :class="store.selectedWorkflow.enabled ? 'btn-outline-danger' : 'btn-success'"
              @click="toggleWorkflowStatus"
              :title="store.selectedWorkflow.enabled ? 'Disable workflow' : 'Enable workflow'"
            >
              <i :class="store.selectedWorkflow.enabled ? 'pause icon' : 'play icon'"/>
              {{ store.selectedWorkflow.enabled ? 'Disable' : 'Enable' }}
            </button>

            <!-- Clone Button (only for configured workflows) -->
            <button
              v-if="store.selectedWorkflow && store.selectedWorkflow.id > 0 && !isInEditMode"
              class="btn btn-outline-secondary"
              @click="cloneWorkflow(store.selectedWorkflow)"
              title="Clone this workflow"
            >
              <i class="copy icon"/>
              Clone
            </button>
          </div>
        </div>

        <div class="editor-content">
          <div class="form" :class="{ 'readonly': !isInEditMode }">
            <div class="field">
              <label>When</label>
              <div class="segment">
                <div class="description">
                  This workflow will run when: <strong>{{ store.selectedWorkflow.display_name }}</strong>
                </div>
              </div>
            </div>

            <!-- Filters Section -->
            <div class="field" v-if="hasAvailableFilters">
              <label>Filters</label>
              <div class="segment">
                <div class="field" v-if="hasFilter('issue_type')">
                  <label>Apply to</label>
                  <select
                    v-if="isInEditMode"
                    class="form-select"
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
              <div class="segment">
                <div class="field" v-if="hasAction('column')">
                  <label>Move to column</label>
                  <select
                    v-if="isInEditMode"
                    class="form-select"
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
                    v-if="isInEditMode"
                    class="form-select"
                    v-model="store.workflowActions.labels"
                    multiple
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
                  <div v-if="isInEditMode" class="form-check">
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
        <div v-if="isInEditMode" class="editor-actions">
          <button class="btn btn-primary" @click="saveWorkflow" :disabled="store.saving">
            <i class="save icon"/>
            Save Workflow
          </button>
          <button
            v-if="store.selectedWorkflow && store.selectedWorkflow.id > 0"
            class="btn btn-danger"
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

.status-indicator .status-disabled {
  color: #dc3545;
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

/* Custom form styles to replace Semantic UI */
.form {
  font-family: inherit;
}

.form .field {
  margin-bottom: 1rem;
}

.form .field label {
  font-weight: 600;
  color: #24292e;
  margin-bottom: 0.5rem;
  display: block;
}

.segment {
  background: #fafbfc;
  border: 1px solid #e1e4e8;
  border-radius: 6px;
  padding: 1rem;
  margin-bottom: 0.5rem;
}

.form-select {
  display: block;
  width: 100%;
  padding: 0.375rem 2.25rem 0.375rem 0.75rem;
  font-size: 1rem;
  font-weight: 400;
  line-height: 1.5;
  color: #212529;
  background-color: #fff;
  background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3e%3cpath fill='none' stroke='%23343a40' stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M2 5l6 6 6-6'/%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 0.75rem center;
  background-size: 16px 12px;
  border: 1px solid #ced4da;
  border-radius: 0.375rem;
  transition: border-color .15s ease-in-out,box-shadow .15s ease-in-out;
}

.form-select:focus {
  border-color: #86b7fe;
  outline: 0;
  box-shadow: 0 0 0 0.25rem rgba(13, 110, 253, 0.25);
}

.form-select[multiple] {
  background-image: none;
  height: auto;
}

.form-check {
  display: block;
  min-height: 1.5rem;
  padding-left: 1.5em;
  margin-bottom: 0.125rem;
}

.form-check input[type="checkbox"] {
  float: left;
  margin-left: -1.5em;
}

/* Button styles to replace Semantic UI */
.btn {
  display: inline-block;
  padding: 0.375rem 0.75rem;
  margin-bottom: 0;
  font-size: 1rem;
  font-weight: 400;
  line-height: 1.5;
  color: #212529;
  text-align: center;
  text-decoration: none;
  vertical-align: middle;
  cursor: pointer;
  background-color: transparent;
  border: 1px solid transparent;
  border-radius: 0.375rem;
  transition: color .15s ease-in-out,background-color .15s ease-in-out,border-color .15s ease-in-out,box-shadow .15s ease-in-out;
}

.btn:hover {
  color: #212529;
  text-decoration: none;
}

.btn:focus {
  outline: 0;
  box-shadow: 0 0 0 0.25rem rgba(13, 110, 253, 0.25);
}

.btn:disabled {
  pointer-events: none;
  opacity: 0.65;
}

.btn-primary {
  color: #fff;
  background-color: #0d6efd;
  border-color: #0d6efd;
}

.btn-primary:hover {
  color: #fff;
  background-color: #0b5ed7;
  border-color: #0a58ca;
}

.btn-primary:focus {
  color: #fff;
  background-color: #0b5ed7;
  border-color: #0a58ca;
  box-shadow: 0 0 0 0.25rem rgba(49, 132, 253, 0.5);
}

.btn-outline-secondary {
  color: #6c757d;
  border-color: #6c757d;
}

.btn-outline-secondary:hover {
  color: #fff;
  background-color: #6c757d;
  border-color: #6c757d;
}

.btn-success {
  color: #fff;
  background-color: #198754;
  border-color: #198754;
}

.btn-success:hover {
  color: #fff;
  background-color: #157347;
  border-color: #146c43;
}

.btn-outline-danger {
  color: #dc3545;
  border-color: #dc3545;
}

.btn-outline-danger:hover {
  color: #fff;
  background-color: #dc3545;
  border-color: #dc3545;
}

.btn-danger {
  color: #fff;
  background-color: #dc3545;
  border-color: #dc3545;
}

.btn-danger:hover {
  color: #fff;
  background-color: #c82333;
  border-color: #bd2130;
}
</style>
