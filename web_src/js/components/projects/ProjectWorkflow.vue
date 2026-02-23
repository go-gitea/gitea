<script lang="ts" setup>
import {onMounted, onUnmounted, useTemplateRef, computed, ref, nextTick, watch} from 'vue';
import {debounce} from 'throttle-debounce';
import {createWorkflowStore} from './WorkflowStore.ts';
import type {WorkflowEvent} from './WorkflowStore.ts';
import {svg} from '../../svg.ts';
import {confirmModal} from '../../features/comp/ConfirmModal.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';
import {contrastColor} from '../../utils/color.ts';

const elRoot = useTemplateRef('elRoot');

const props = defineProps<{
  projectLink: string;
  eventID: string;
  locale: {
    defaultWorkflows: string;
    moveToColumn: string;
    viewWorkflowConfiguration: string;
    configureWorkflow: string;
    when: string;
    runWhen: string;
    filters: string;
    applyTo: string;
    whenMovedFromColumn: string;
    whenMovedToColumn: string;
    onlyIfHasLabels: string;
    actions: string;
    addLabels: string;
    removeLabels: string;
    anyLabel: string;
    anyColumn: string;
    issueState: string;
    none: string;
    noChange: string;
    edit: string;
    delete: string;
    save: string;
    clone: string;
    cancel: string;
    disable: string;
    disabled: string;
    enabled: string;
    enable: string;
    issuesAndPullRequests: string;
    issuesOnly: string;
    pullRequestsOnly: string;
    selectColumn: string;
    closeIssue: string;
    reopenIssue: string;
    saveWorkflowFailed: string;
    updateWorkflowFailed: string;
    deleteWorkflowFailed: string;
    atLeastOneActionRequired: string;
  },
}>();

const store = createWorkflowStore(props);

type WorkflowListItem = WorkflowEvent & {isConfigured?: boolean};

// Track edit state directly on workflow objects
const previousSelection = ref<{selectedItem: string | null, selectedWorkflow: WorkflowEvent | null} | null>(null);

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
const setEditMode = (enabled: boolean) => {
  if (store.selectedWorkflow) {
    store.selectedWorkflow._isEditing = enabled;
  }
};

const showCancelButton = computed(() => {
  if (!store.selectedWorkflow) return false;
  if (store.selectedWorkflow.id > 0) return true;
  const eventId = store.selectedWorkflow.eventId ?? '';
  return typeof eventId === 'string' && eventId.startsWith('clone-');
});

const isTemporaryWorkflow = (workflow?: WorkflowEvent | null) => {
  if (!workflow) return false;
  if (workflow.id > 0) return false;
  const eventId = typeof workflow.eventId === 'string' ? workflow.eventId : '';
  return eventId.startsWith('clone-') || eventId.startsWith('new-');
};

const removeTemporaryWorkflow = (workflow?: WorkflowEvent | null) => {
  if (!workflow || !isTemporaryWorkflow(workflow)) return;

  const eventId = workflow.eventId;
  const tempIndex = store.workflowEvents.findIndex((w: WorkflowEvent) => w.eventId === eventId);
  if (tempIndex >= 0) {
    store.workflowEvents.splice(tempIndex, 1);
  }

  if (typeof store.clearDraft === 'function') {
    store.clearDraft(eventId);
  }
};

const toggleEditMode = () => {
  if (isInEditMode.value) {
    // Canceling edit mode
    const canceledWorkflow = store.selectedWorkflow;
    const hadTemporarySelection = isTemporaryWorkflow(canceledWorkflow);

    if (hadTemporarySelection) {
      removeTemporaryWorkflow(canceledWorkflow);
    }

    if (previousSelection.value) {
      // If there was a previous selection, return to it
      // Restore previous selection
      store.selectedItem = previousSelection.value.selectedItem;
      store.selectedWorkflow = previousSelection.value.selectedWorkflow;
      if (previousSelection.value.selectedWorkflow) {
        store.loadWorkflowData(previousSelection.value.selectedWorkflow.eventId);
      }
      previousSelection.value = null;
    } else if (hadTemporarySelection) {
      // If we removed a temporary item but have no previous selection, fall back to first workflow
      const fallback = store.workflowEvents.find((w: WorkflowEvent) => {
        if (!canceledWorkflow) return false;
        const baseType = canceledWorkflow.workflowEvent;
        return baseType && (w.workflowEvent === baseType || w.eventId === baseType);
      }) || store.workflowEvents[0];
      if (fallback) {
        store.selectedItem = fallback.eventId;
        store.selectedWorkflow = fallback;
        store.loadWorkflowData(fallback.eventId);
      } else {
        store.selectedItem = null;
        store.selectedWorkflow = null;
      }
    }
    setEditMode(false);
  } else {
    // Entering edit mode - store current selection
    previousSelection.value = {
      selectedItem: store.selectedItem,
      selectedWorkflow: store.selectedWorkflow ? {...store.selectedWorkflow} : null,
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
  const currentSelection = store.selectedWorkflow;
  if (!currentSelection) return;

  if (!await confirmModal({content: 'Are you sure you want to delete this workflow?', confirmButtonColor: 'red'})) {
    return;
  }

  // If deleting a temporary workflow (new or cloned, unsaved), just remove from list
  if (currentSelection.id === 0) {
    const tempIndex = store.workflowEvents.findIndex((w: WorkflowEvent) =>
      w.eventId === currentSelection.eventId,
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
  const sameEventWorkflows = store.workflowEvents.filter((w: WorkflowEvent) =>
    (w.workflowEvent === currentSelection.workflowEvent)
  );

  let workflowToSelect: WorkflowListItem | null = null;

  if (sameEventWorkflows.length > 0) {
    // Prefer configured workflows over placeholders
    const configured = sameEventWorkflows.find((w: WorkflowEvent) => w.isConfigured || w.id > 0);
    workflowToSelect = configured || sameEventWorkflows[0];
  }

  // If no same-type workflow found, select the first available workflow
  if (!workflowToSelect && store.workflowEvents.length > 0) {
    // Try to find any configured workflow first
    const anyConfigured = store.workflowEvents.find((w: WorkflowEvent) => w.isConfigured || w.id > 0);
    workflowToSelect = anyConfigured || store.workflowEvents[0];
  }

  if (workflowToSelect) {
    await selectWorkflowItem(workflowToSelect);

    // If selected workflow is unconfigured, automatically enter edit mode
    if (!workflowToSelect.isConfigured && workflowToSelect.id === 0) {
      previousSelection.value = null;
      setEditMode(true);
      return; // Early return to avoid setting edit mode to false below
    }
  } else {
    // No workflows at all (shouldn't happen), clear selection
    store.selectedItem = null;
    store.selectedWorkflow = null;
    window.history.pushState({}, '', `${props.projectLink}/workflows`);
  }

  // Clear previous selection and exit edit mode
  previousSelection.value = null;
  setEditMode(false);
};

const cloneWorkflow = (sourceWorkflow?: WorkflowEvent | null) => {
  if (!sourceWorkflow) return;

  // Generate a unique temporary ID for the cloned workflow
  const tempId = `${sourceWorkflow.workflowEvent}`;

  // Extract base name without any parenthetical descriptions
  const baseName = (sourceWorkflow.displayName || sourceWorkflow.workflowEvent || sourceWorkflow.eventId)
    .replace(/\s*\([^)]*\)\s*/g, '');

  // Create a new workflow object based on the source
  const clonedWorkflow = {
    id: 0, // New workflow
    eventId: tempId,
    displayName: `${baseName} (Copy)`,
    workflowEvent: sourceWorkflow.workflowEvent,
    capabilities: sourceWorkflow.capabilities,
    filters: JSON.parse(JSON.stringify(sourceWorkflow.filters || [])), // Deep clone
    actions: JSON.parse(JSON.stringify(sourceWorkflow.actions || [])), // Deep clone
    enabled: false, // Cloned workflows start disabled
    isConfigured: false, // Mark as new/unsaved
  };

  // Insert cloned workflow right after the source workflow (keep same type together)
  const sourceIndex = store.workflowEvents.findIndex((w: WorkflowEvent) => w.eventId === sourceWorkflow.eventId);
  if (sourceIndex >= 0) {
    store.workflowEvents.splice(sourceIndex + 1, 0, clonedWorkflow);
  } else {
    // Fallback: add to end if source not found
    store.workflowEvents.push(clonedWorkflow);
  }

  // Remember the source so cancel can return to it
  previousSelection.value = {
    selectedItem: store.selectedItem,
    selectedWorkflow: store.selectedWorkflow ? {...store.selectedWorkflow} : {...sourceWorkflow},
  };

  // Select the cloned workflow and enter edit mode
  store.selectedItem = tempId;
  store.selectedWorkflow = clonedWorkflow;

  // Load the workflow data into the form
  store.loadWorkflowData(tempId);

  // Enter edit mode
  setEditMode(true);

  // Update URL
  const newUrl = `${props.projectLink}/workflows/${tempId}`;
  window.history.pushState({eventId: tempId}, '', newUrl);
};

const selectWorkflowEvent = async (event: WorkflowEvent) => {
  // Prevent rapid successive clicks
  if (store.loading) return;

  // If already selected, do nothing (keep selection active)
  if (store.selectedItem === event.eventId) {
    return;
  }

  try {
    store.selectedItem = event.eventId;
    store.selectedWorkflow = event;

    // Wait for DOM update before proceeding
    await nextTick();

    await store.loadWorkflowData(event.eventId);

    // Update URL without page reload
    const newUrl = `${props.projectLink}/workflows/${event.eventId}`;
    window.history.pushState({eventId: event.eventId}, '', newUrl);
  } catch (error) {
    console.error('Error selecting workflow event:', error);
    // On error, try to select the first available workflow instead of clearing
    const items = workflowList.value;
    if (items.length > 0 && items[0] !== event) {
      selectWorkflowItem(items[0]);
    }
  }
};

const saveWorkflow = async () => {
  await store.saveWorkflow();
  // The store.saveWorkflow already handles reloading events

  // Clear previous selection after successful save
  previousSelection.value = null;
  setEditMode(false);
};

const isWorkflowConfigured = (event: WorkflowEvent) => {
  // Check if the event_id is a number (saved workflow ID) or if it has id > 0
  return !Number.isNaN(parseInt(event.eventId)) || (event.id !== undefined && event.id > 0);
};

// Get flat list of all workflows - use cached data to prevent frequent recomputation
const workflowList = computed<WorkflowListItem[]>(() => {
  // Use a stable reference to prevent unnecessary DOM updates
  const workflows = store.workflowEvents;
  if (!workflows || workflows.length === 0) {
    return [];
  }

  return workflows.map((workflow: WorkflowEvent) => ({
    ...workflow,
    isConfigured: isWorkflowConfigured(workflow),
    displayName: workflow.displayName || workflow.workflowEvent || workflow.eventId,
  }));
});

const selectWorkflowItem = async (item: WorkflowListItem) => {
  if (store.loading) return;

  previousSelection.value = null; // Clear previous selection when manually selecting
  // Don't reset edit mode when switching - each workflow keeps its own state

  // Wait for DOM update to prevent conflicts
  await nextTick();

  if (item.isConfigured) {
    // This is a configured workflow, select it
    await selectWorkflowEvent(item);
  } else {
    // This is an unconfigured event - check if we already have a workflow object for it
    const existingWorkflow = store.workflowEvents.find((w: WorkflowEvent) =>
      w.id === 0 && w.workflowEvent === item.workflowEvent,
    );

    const workflowToSelect = existingWorkflow || item;
    await selectWorkflowEvent(workflowToSelect);

    // Update URL for workflow
    const newUrl = `${props.projectLink}/workflows/${item.workflowEvent}`;
    window.history.pushState({eventId: item.workflowEvent}, '', newUrl);
  }
};

const debouncedSelectWorkflowItem = debounce(150, (item: WorkflowListItem) => {
  void selectWorkflowItem(item);
});

const hasAvailableFilters = computed(() => {
  return (store.selectedWorkflow?.capabilities?.availableFilters?.length ?? 0) > 0;
});

const hasFilter = (filterType: any) => {
  return store.selectedWorkflow?.capabilities?.availableFilters?.includes(filterType);
};

const hasAction = (actionType: any) => {
  return store.selectedWorkflow?.capabilities?.availableActions?.includes(actionType);
};

// Toggle label selection for add_labels, remove_labels, or filter_labels
const toggleLabel = (type: string, labelId: any) => {
  let labels;
  if (type === 'filter_labels') {
    labels = store.workflowFilters.labels;
  } else if (type === 'add_labels') {
    labels = (store.workflowActions as any)['addLabels'];
  } else if (type === 'remove_labels') {
    labels = (store.workflowActions as any)['removeLabels'];
  }
  const index = labels.indexOf(labelId);
  if (index > -1) {
    labels.splice(index, 1);
  } else {
    labels.push(labelId);
  }
};

// Calculate text color based on background color for better contrast
const getLabelTextColor = (hexColor: any) => {
  return contrastColor(hexColor);
};

const getStatusClass = (item: WorkflowListItem) => {
  if (!item.isConfigured) {
    return 'status-inactive'; // Gray dot for unconfigured
  }

  // For configured workflows, check enabled status
  if (item.enabled === false) {
    return 'status-disabled'; // Red dot for disabled
  }

  return 'status-active'; // Green dot for enabled
};

const isItemSelected = (item: WorkflowListItem) => {
  if (!store.selectedItem) return false;

  if (item.isConfigured || item.id === 0) {
    // For configured workflows or temporary workflows (new), match by event_id
    return store.selectedItem === item.eventId;
  }
    // For unconfigured events, match by workflow_event
  return store.selectedItem === item.workflowEvent;
};

// Get display name for workflow with numbering for same types
const getWorkflowDisplayName = (item: WorkflowListItem, _index: number) => {
  const list = workflowList.value;
  const displayName = item.displayName || item.workflowEvent || item.eventId || '';

  // Find all workflows of the same type
  const sameTypeWorkflows = list.filter((w: WorkflowListItem) =>
    w.workflowEvent === item.workflowEvent &&
    (w.isConfigured || w.id === 0) // Only count configured workflows
  );

  // If there's only one of this type, return the display name as-is
  if (sameTypeWorkflows.length <= 1) {
    return displayName;
  }

  // Find the index of this workflow among same-type workflows
  const sameTypeIndex = sameTypeWorkflows.findIndex((w: WorkflowListItem) => w.eventId === item.eventId);

  // Extract base name without filter summary (remove anything in parentheses)
  const baseName = displayName.replace(/\s*\([^)]*\)\s*$/g, '');

  // Add numbering
  return `${baseName} #${sameTypeIndex + 1}`;
};


const getCurrentDraftKey = () => {
  if (!store.selectedWorkflow) return null;
  return store.selectedWorkflow.eventId || store.selectedWorkflow.workflowEvent;
};

const persistDraftState = () => {
  const draftKey = getCurrentDraftKey();
  if (!draftKey) return;
  store.updateDraft(draftKey, store.workflowFilters, store.workflowActions);
};

// Initialize Fomantic UI dropdowns for label selection
const initLabelDropdowns = () => {
  const dropdowns = elRoot.value?.querySelectorAll('.ui.dropdown');
  if (dropdowns) {
    dropdowns.forEach((dropdown) => {
      fomanticQuery(dropdown).dropdown({
        action: 'nothing', // Don't hide on selection for multiple selection
        fullTextSearch: true,
      });
    });
  }
};

// Watch for edit mode changes to initialize dropdowns
watch(isInEditMode, async (newVal) => {
  if (newVal) {
    await nextTick();
    initLabelDropdowns();
  }
});

watch(() => store.workflowFilters, () => {
  persistDraftState();
}, {deep: true});

watch(() => store.workflowActions, () => {
  persistDraftState();
}, {deep: true});

onMounted(async () => {
  // Load all necessary data
  store.workflowEvents = await store.loadEvents();
  await store.loadProjectColumns();
  await store.loadProjectLabels();

  // Add native event listener to prevent conflicts with Gitea
  await nextTick();
  const rootEl = elRoot.value;
  const workflowItemsContainer = rootEl?.querySelector<HTMLElement>('.workflow-items');
  if (workflowItemsContainer) {
    workflowClickHandler = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      const workflowItem = target?.closest('.workflow-item');
      if (workflowItem) {
        event.preventDefault();
        event.stopPropagation();
        const itemData = workflowItem.getAttribute('data-workflow-item');
        if (itemData) {
          try {
            const item = JSON.parse(itemData) as WorkflowListItem;
            if (!store.loading) {
              debouncedSelectWorkflowItem(item);
            }
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
    const selectedEvent = store.workflowEvents.find((e: WorkflowEvent) => e.eventId === props.eventID);
    if (selectedEvent) {
      // Found existing configured workflow
      store.selectedItem = props.eventID;
      store.selectedWorkflow = selectedEvent;
      await store.loadWorkflowData(props.eventID);
    } else {
      // Check if eventID matches a base event type (unconfigured workflow)
      const items = workflowList.value;
      const matchingUnconfigured = items.find((item: WorkflowListItem) =>
        !item.isConfigured && (item.workflowEvent === props.eventID || item.eventId === props.eventID),
      );
      if (matchingUnconfigured) {
        // Select the placeholder workflow for this base event type
        store.selectedItem = null;
        await selectWorkflowEvent(matchingUnconfigured);
      } else {
        // Fallback: select first available item
        if (items.length > 0) {
          const firstConfigured = items.find((item: WorkflowListItem) => item.isConfigured);
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
      const firstConfigured = items.find((item: WorkflowListItem) => item.isConfigured);

      if (firstConfigured) {
        // Select first configured workflow
        selectWorkflowItem(firstConfigured);
      } else {
        // No configured workflows, select first item
        selectWorkflowItem(items[0]);
      }
    }
  }

  elRoot.value?.closest('.is-loading')?.classList?.remove('is-loading');

  window.addEventListener('popstate', popstateHandler);
});

// Define popstateHandler at component level
const popstateHandler = (e: PopStateEvent) => {
  if (e.state?.eventId) {
    // Handle browser back/forward navigation
    const event = store.workflowEvents.find((ev: WorkflowEvent) => ev.eventId === e.state.eventId);
    if (event) {
      void selectWorkflowEvent(event);
    } else {
      // Check if it's a base event type
      const items = workflowList.value;
      const matchingUnconfigured = items.find((item: WorkflowListItem) =>
        !item.isConfigured && (item.workflowEvent === e.state.eventId || item.eventId === e.state.eventId),
      );
      if (matchingUnconfigured) {
        void selectWorkflowEvent(matchingUnconfigured);
      }
    }
  }
};

// Store reference to cleanup event listener
let workflowClickHandler: ((e: MouseEvent) => void) | null = null;

onUnmounted(() => {
  // Clean up resources
  debouncedSelectWorkflowItem.cancel();
  window.removeEventListener('popstate', popstateHandler);

  // Remove native click event listener
  const workflowItemsContainer = elRoot.value?.querySelector<HTMLElement>('.workflow-items');
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
        <h3>{{ locale.defaultWorkflows }}</h3>
      </div>

      <div class="sidebar-content">
        <!-- Flat Workflow List -->
        <div class="workflow-items">
          <div
            v-for="(item, index) in workflowList"
            :key="`workflow-${item.eventId}-${item.isConfigured ? 'configured' : 'unconfigured'}`"
            class="workflow-item"
            :class="{ active: isItemSelected(item) }"
            :data-workflow-item="JSON.stringify(item)"
          >
            <div class="workflow-content">
              <div class="workflow-info">
                <span class="status-indicator">
                  <!-- eslint-disable-next-line vue/no-v-html -->
                  <span v-html="svg('octicon-dot-fill')" :class="getStatusClass(item)"/>
                </span>
                <div class="workflow-details">
                  <div class="workflow-title">
                    {{ getWorkflowDisplayName(item, index) }}
                  </div>
                  <div v-if="item.summary" class="workflow-subtitle">
                    {{ item.summary }}
                  </div>
                </div>
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
        </div>
      </div>

      <!-- Workflow Editor -->
      <div v-else class="workflow-editor">
        <div class="editor-header">
          <div class="editor-title">
            <h2>
              <i class="settings icon"/>
              {{ store.selectedWorkflow.displayName }}
              <span
                v-if="store.selectedWorkflow.id > 0 && !isInEditMode"
                class="workflow-status"
                :class="store.selectedWorkflow.enabled ? 'status-enabled' : 'status-disabled'"
              >
                {{ store.selectedWorkflow.enabled ? locale.enabled : locale.disabled }}
              </span>
            </h2>
            <p v-if="store.selectedWorkflow.id === 0">{{ locale.configureWorkflow }}</p>
            <p v-else-if="isInEditMode">{{ locale.configureWorkflow }}</p>
            <p v-else>{{ locale.viewWorkflowConfiguration }}</p>
          </div>
          <div class="editor-actions-header">
            <!-- Edit Mode Buttons -->
            <template v-if="isInEditMode">
              <!-- Cancel Button -->
              <button
                v-if="showCancelButton"
                class="ui small button"
                @click="toggleEditMode"
              >
                <i class="times icon"/>
                {{ locale.cancel }}
              </button>

              <!-- Save Button -->
              <button
                class="ui small primary button"
                @click="saveWorkflow"
                :disabled="store.saving"
              >
                <i class="save icon"/>
                {{ locale.save }}
              </button>

              <!-- Delete Button (only for configured workflows) -->
              <button
                v-if="store.selectedWorkflow && store.selectedWorkflow.id > 0"
                class="ui small red button"
                @click="deleteWorkflow"
              >
                <i class="trash icon"/>
                {{ locale.delete }}
              </button>
            </template>

            <!-- View Mode Buttons (only for configured workflows) -->
            <template v-else-if="store.selectedWorkflow && store.selectedWorkflow.id > 0">
              <!-- Edit Button -->
              <button
                class="ui small primary button"
                @click="toggleEditMode"
              >
                <i class="edit icon"/>
                {{ locale.edit }}
              </button>

              <!-- Enable/Disable Button -->
              <button
                class="ui small button"
                :class="store.selectedWorkflow.enabled ? 'basic red' : 'green'"
                @click="toggleWorkflowStatus"
              >
                <i :class="store.selectedWorkflow.enabled ? 'pause icon' : 'play icon'"/>
                {{ store.selectedWorkflow.enabled ? locale.disable : locale.enable }}
              </button>

              <!-- Clone Button -->
              <button
                class="ui small button"
                @click="cloneWorkflow(store.selectedWorkflow)"
                title="Clone this workflow"
              >
                <i class="copy icon"/>
                {{ locale.clone }}
              </button>
            </template>
          </div>
        </div>

        <div class="editor-content">
          <div class="form" :class="{ 'readonly': !isInEditMode }">
            <div class="field">
              <label>{{ locale.when }}</label>
              <div class="segment">
                <div class="description">
                  {{ locale.runWhen }}<strong>{{ store.selectedWorkflow.displayName }}</strong>
                </div>
              </div>
            </div>

            <!-- Filters Section -->
            <div class="field" v-if="hasAvailableFilters">
              <label>{{ locale.filters }}</label>
              <div class="segment">
                <div class="field" v-if="hasFilter('issue_type')">
                  <label>{{ locale.applyTo }}</label>
                  <select
                    v-if="isInEditMode"
                    class="column-select"
                    v-model="store.workflowFilters.issueType"
                  >
                    <option value="">{{ locale.issuesAndPullRequests }}</option>
                    <option value="issue">{{ locale.issuesOnly }}</option>
                    <option value="pull_request">{{ locale.pullRequestsOnly }}</option>
                  </select>
                  <div v-else class="readonly-value">
                    {{ store.workflowFilters.issueType === 'issue' ? locale.issuesOnly :
                      store.workflowFilters.issueType === 'pull_request' ? locale.pullRequestsOnly :
                      locale.issuesAndPullRequests }}
                  </div>
                </div>

                <div class="field" v-if="hasFilter('source_column')">
                  <label>{{ locale.whenMovedFromColumn }}</label>
                  <select
                    v-if="isInEditMode"
                    v-model="store.workflowFilters.sourceColumn"
                    class="column-select"
                  >
                    <option value="">{{ locale.anyColumn }}</option>
                    <option v-for="column in store.projectColumns" :key="column.id" :value="String(column.id)">
                      {{ column.title }}
                    </option>
                  </select>
                  <div v-else class="readonly-value">
                    {{ store.projectColumns.find(c => String(c.id) === store.workflowFilters.sourceColumn)?.title || locale.anyColumn }}
                  </div>
                </div>

                <div class="field" v-if="hasFilter('target_column')">
                  <label>{{ locale.whenMovedToColumn }}</label>
                  <select
                    v-if="isInEditMode"
                    v-model="store.workflowFilters.targetColumn"
                    class="column-select"
                  >
                    <option value="">{{ locale.anyColumn }}</option>
                    <option v-for="column in store.projectColumns" :key="column.id" :value="String(column.id)">
                      {{ column.title }}
                    </option>
                  </select>
                  <div v-else class="readonly-value">
                    {{ store.projectColumns.find(c => String(c.id) === store.workflowFilters.targetColumn)?.title || locale.anyColumn }}
                  </div>
                </div>

                <div class="field" v-if="hasFilter('labels')">
                  <label>{{ locale.onlyIfHasLabels }}</label>
                  <div v-if="isInEditMode" class="ui fluid multiple search selection dropdown custom label-dropdown">
                    <input type="hidden" :value="store.workflowFilters.labels.join(',')">
                    <i class="dropdown icon"/>
                    <div class="text" :class="{ default: !store.workflowFilters.labels?.length }">
                      <span v-if="!store.workflowFilters.labels?.length">{{ locale.anyLabel }}</span>
                      <template v-else>
                        <span
                          v-for="labelId in store.workflowFilters.labels" :key="labelId"
                          class="ui label"
                          :style="`background-color: ${store.projectLabels.find(l => String(l.id) === labelId)?.color}; color: ${getLabelTextColor(store.projectLabels.find(l => String(l.id) === labelId)?.color)}`"
                        >
                          {{ store.projectLabels.find(l => String(l.id) === labelId)?.name }}
                        </span>
                      </template>
                    </div>
                    <div class="menu">
                      <div
                        class="item" v-for="label in store.projectLabels" :key="label.id"
                        :data-value="String(label.id)"
                        @click.prevent="toggleLabel('filter_labels', String(label.id))"
                        :class="{ active: store.workflowFilters.labels.includes(String(label.id)), selected: store.workflowFilters.labels.includes(String(label.id)) }"
                      >
                        <span class="ui label" :style="`background-color: ${label.color}; color: ${getLabelTextColor(label.color)}`">
                          {{ label.name }}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div v-else class="ui list labels-list">
                    <span v-if="!store.workflowFilters.labels?.length" class="text-muted">{{ locale.anyLabel }}</span>
                    <span
                      v-for="labelId in store.workflowFilters.labels" :key="labelId"
                      class="ui label"
                      :style="`background-color: ${store.projectLabels.find(l => String(l.id) === labelId)?.color}; color: ${getLabelTextColor(store.projectLabels.find(l => String(l.id) === labelId)?.color)}`"
                    >
                      {{ store.projectLabels.find(l => String(l.id) === labelId)?.name }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Actions Section -->
            <div class="field">
              <label>{{ locale.actions }}</label>
              <div class="segment">
                <div class="field" v-if="hasAction('column')">
                  <label>{{ locale.moveToColumn }}</label>
                  <select
                    v-if="isInEditMode"
                    v-model="store.workflowActions.column"
                    class="column-select"
                  >
                    <option value="">{{ locale.selectColumn }}</option>
                    <option v-for="column in store.projectColumns" :key="column.id" :value="String(column.id)">
                      {{ column.title }}
                    </option>
                  </select>
                  <div v-else class="readonly-value">
                    {{ store.projectColumns.find(c => String(c.id) === store.workflowActions.column)?.title || locale.none }}
                  </div>
                </div>

                <div class="field" v-if="hasAction('add_labels')">
                  <label>{{ locale.addLabels }}</label>
                  <div v-if="isInEditMode" class="ui fluid multiple search selection dropdown custom label-dropdown">
                    <input type="hidden" :value="store.workflowActions.addLabels.join(',')">
                    <i class="dropdown icon"/>
                    <div class="text" :class="{ default: !store.workflowActions.addLabels?.length }">
                      <span v-if="!store.workflowActions.addLabels?.length">{{ locale.none }}</span>
                      <template v-else>
                        <span
                          v-for="labelId in store.workflowActions.addLabels" :key="labelId"
                          class="ui label"
                          :style="`background-color: ${store.projectLabels.find(l => String(l.id) === labelId)?.color}; color: ${getLabelTextColor(store.projectLabels.find(l => String(l.id) === labelId)?.color)}`"
                        >
                          {{ store.projectLabels.find(l => String(l.id) === labelId)?.name }}
                        </span>
                      </template>
                    </div>
                    <div class="menu">
                      <div
                        class="item" v-for="label in store.projectLabels" :key="label.id"
                        :data-value="String(label.id)"
                        @click.prevent="toggleLabel('add_labels', String(label.id))"
                        :class="{ active: store.workflowActions.addLabels.includes(String(label.id)), selected: store.workflowActions.addLabels.includes(String(label.id)) }"
                      >
                        <span class="ui label" :style="`background-color: ${label.color}; color: ${getLabelTextColor(label.color)}`">
                          {{ label.name }}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div v-else class="ui list labels-list">
                    <span v-if="!store.workflowActions.addLabels?.length" class="text-muted">{{ locale.none }}</span>
                    <span
                      v-for="labelId in store.workflowActions.addLabels" :key="labelId"
                      class="ui label"
                      :style="`background-color: ${store.projectLabels.find(l => String(l.id) === labelId)?.color}; color: ${getLabelTextColor(store.projectLabels.find(l => String(l.id) === labelId)?.color)}`"
                    >
                      {{ store.projectLabels.find(l => String(l.id) === labelId)?.name }}
                    </span>
                  </div>
                </div>

                <div class="field" v-if="hasAction('remove_labels')">
                  <label>{{ locale.removeLabels }}</label>
                  <div v-if="isInEditMode" class="ui fluid multiple search selection dropdown custom label-dropdown">
                    <input type="hidden" :value="store.workflowActions.removeLabels.join(',')">
                    <i class="dropdown icon"/>
                    <div class="text" :class="{ default: !store.workflowActions.removeLabels?.length }">
                      <span v-if="!store.workflowActions.removeLabels?.length">{{ locale.none }}</span>
                      <template v-else>
                        <span
                          v-for="labelId in store.workflowActions.removeLabels" :key="labelId"
                          class="ui label"
                          :style="`background-color: ${store.projectLabels.find(l => String(l.id) === labelId)?.color}; color: ${getLabelTextColor(store.projectLabels.find(l => String(l.id) === labelId)?.color)}`"
                        >
                          {{ store.projectLabels.find(l => String(l.id) === labelId)?.name }}
                        </span>
                      </template>
                    </div>
                    <div class="menu">
                      <div
                        class="item" v-for="label in store.projectLabels" :key="label.id"
                        :data-value="String(label.id)"
                        @click.prevent="toggleLabel('remove_labels', String(label.id))"
                        :class="{ active: store.workflowActions.removeLabels.includes(String(label.id)), selected: store.workflowActions.removeLabels.includes(String(label.id)) }"
                      >
                        <span class="ui label" :style="`background-color: ${label.color}; color: ${getLabelTextColor(label.color)}`">
                          {{ label.name }}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div v-else class="ui list labels-list">
                    <span v-if="!store.workflowActions.removeLabels?.length" class="text-muted">{{ locale.none }}</span>
                    <span
                      v-for="labelId in store.workflowActions.removeLabels" :key="labelId"
                      class="ui label"
                      :style="`background-color: ${store.projectLabels.find(l => String(l.id) === labelId)?.color}; color: ${getLabelTextColor(store.projectLabels.find(l => String(l.id) === labelId)?.color)}`"
                    >
                      {{ store.projectLabels.find(l => String(l.id) === labelId)?.name }}
                    </span>
                  </div>
                </div>

                <div class="field" v-if="hasAction('issue_state')">
                  <label for="issue-state-action">{{ locale.issueState }}</label>
                  <select
                    v-if="isInEditMode"
                    id="issue-state-action"
                    class="column-select"
                    v-model="store.workflowActions.issueState"
                  >
                    <option value="">{{ locale.noChange }}</option>
                    <option value="close">{{ locale.closeIssue }}</option>
                    <option value="reopen">{{ locale.reopenIssue }}</option>
                  </select>
                  <div v-else class="readonly-value">
                    {{ store.workflowActions.issueState === 'close' ? locale.closeIssue :
                      store.workflowActions.issueState === 'reopen' ? locale.reopenIssue : locale.noChange }}
                  </div>
                </div>
              </div>
            </div>
          </div>
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
  border: 1px solid var(--color-secondary);
  border-radius: 8px;
  overflow: hidden;
  background: var(--color-body);
}

.workflow-sidebar {
  width: 350px;
  flex-shrink: 0;
  background: var(--color-secondary-bg);
  border-right: 1px solid var(--color-secondary);
  display: flex;
  flex-direction: column;
}

.workflow-main {
  flex: 1;
  background: var(--color-body);
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* Sidebar */
.sidebar-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--color-secondary);
  background: var(--color-secondary-bg);
}

.sidebar-header h3 {
  margin: 0;
  color: var(--color-text);
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
  background: var(--color-hover);
}

.workflow-item.active {
  background: var(--color-active);
  border-left: 3px solid var(--color-primary);
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
  min-width: 0; /* Allow text truncation */
}

.workflow-details {
  flex: 1;
  min-width: 0; /* Allow text truncation */
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.workflow-title {
  font-weight: 500;
  color: var(--color-text);
  font-size: 0.9rem;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workflow-subtitle {
  font-size: 0.75rem;
  color: var(--color-text-light-2);
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-style: italic;
}

.status-indicator .status-active {
  color: var(--color-green);
  font-size: 0.75rem;
}

.status-indicator .status-inactive {
  color: var(--color-text-light-3);
  font-size: 0.75rem;
}

.status-indicator .status-disabled {
  color: var(--color-red);
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
  color: var(--color-text-light-3);
}

.placeholder-content h3 {
  color: var(--color-text);
  margin-bottom: 0.5rem;
  font-weight: 600;
}

.placeholder-content p {
  color: var(--color-text-light-2);
  margin-bottom: 2rem;
  line-height: 1.5;
}

/* Editor */
.workflow-editor {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 1.5rem;
  border-bottom: 1px solid var(--color-secondary);
  background: var(--color-box-header);
}

.editor-title h2 {
  margin: 0 0 0.25rem 0;
  color: var(--color-text);
  font-size: 1.25rem;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.editor-title p {
  margin: 0;
  color: var(--color-text-light-2);
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
  min-height: 0;
}

.editor-content .field {
  margin-bottom: 1.5rem;
}

.editor-content .field label {
  font-weight: 600;
  color: var(--color-text);
  margin-bottom: 0.5rem;
  display: block;
}

.editor-content .ui.segment {
  background: var(--color-box-header);
  border: 1px solid var(--color-secondary);
  padding: 1rem;
  margin-bottom: 0.5rem;
}

.editor-content .description {
  color: var(--color-text-light-2);
  font-size: 0.9rem;
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
    border-bottom: 1px solid var(--color-secondary);
  }

  .editor-header {
    flex-direction: column;
    gap: 1rem;
    align-items: stretch;
  }

  .editor-content {
    padding: 1rem;
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

  .editor-actions-header {
    flex-wrap: wrap;
  }

  .editor-actions-header button {
    flex: 1 1 auto;
    min-width: 80px;
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
  background: var(--color-success-bg);
  color: var(--color-success-text);
  border: 1px solid var(--color-success-border);
}

.workflow-status.status-disabled {
  background: var(--color-error-bg);
  color: var(--color-error-text);
  border: 1px solid var(--color-error-border);
}

/* Readonly form styles */
.ui.form.readonly {
  pointer-events: none;
}

.readonly-value {
  background: var(--color-secondary-bg);
  padding: 0.5rem;
  border: 1px solid var(--color-secondary);
  border-radius: 4px;
  color: var(--color-text);
  font-weight: 500;
}

.readonly-value label {
  font-weight: 600;
  margin-bottom: 0.25rem;
  display: block;
}

.readonly-value div {
  color: var(--color-text-light-2);
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
  color: var(--color-text);
  margin-bottom: 0.5rem;
  display: block;
}

.segment {
  background: var(--color-box-header);
  border: 1px solid var(--color-secondary);
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
  color: var(--color-text);
  background-color: var(--color-input-background);
  background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3e%3cpath fill='none' stroke='%23343a40' stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M2 5l6 6 6-6'/%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 0.75rem center;
  background-size: 16px 12px;
  border: 1px solid var(--color-input-border);
  border-radius: 0.375rem;
  transition: border-color .15s ease-in-out,box-shadow .15s ease-in-out;
}

.form-select:focus {
  border-color: var(--color-primary);
  outline: 0;
  box-shadow: 0 0 0 0.25rem var(--color-primary-alpha-30);
}

.form-select[multiple] {
  background-image: none;
  height: auto;
}

/* Column select styling */
.column-select {
  width: 100%;
  padding: 0.67857143em 1em;
  border: 1px solid var(--color-input-border);
  border-radius: 0.28571429rem;
  font-size: 1em;
  line-height: 1.21428571em;
  min-height: 2.71428571em;
  background-color: var(--color-input-background);
  color: var(--color-input-text);
  transition: border-color 0.1s ease, box-shadow 0.1s ease;
}

.column-select:focus {
  border-color: var(--color-primary);
  outline: none;
  box-shadow: 0 0 0 0 var(--color-primary-alpha-30) inset;
}

/* Label selector styles */
.label-dropdown.ui.dropdown .menu > .item.active,
.label-dropdown.ui.dropdown .menu > .item.selected {
  background: var(--color-active);
  font-weight: normal;
}

.label-dropdown.ui.dropdown .menu > .item .ui.label {
  margin: 0;
}

.label-dropdown.ui.dropdown > .text > .ui.label {
  margin: 0.125rem;
}

.text-muted {
  color: var(--color-text-light-2);
}
</style>
