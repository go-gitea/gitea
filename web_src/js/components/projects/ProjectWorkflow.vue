<script lang="ts" setup>
import {onMounted, useTemplateRef} from 'vue';
import {createWorkflowStore} from './WorkflowStore.ts';
import {svg} from '../../svg.ts';

const elRoot = useTemplateRef('elRoot');

const props = defineProps({
  projectLink: {type: String, required: true},
  eventID: {type: String, required: true},
});

const store = createWorkflowStore(props);

const selectWorkflowEvent = (event) => {
  store.selectedItem = event.event_id;
  store.selectedWorkflow = event;
  store.loadWorkflowData(event.event_id);

  // Update URL without page reload
  const newUrl = `${props.projectLink}/workflows/${event.event_id}`;
  window.history.pushState({eventId: event.event_id}, '', newUrl);
};

const saveWorkflow = async () => {
  await store.saveWorkflow();
};

const resetWorkflow = () => {
  store.resetWorkflowData();
};

const isWorkflowConfigured = (event) => {
  // Check if the event_id is a number (saved workflow ID) vs UUID (unconfigured)
  // If it's a number, it means the workflow has been saved to database
  return !isNaN(parseInt(event.event_id));
};

onMounted(async () => {
  store.workflowEvents = await store.loadEvents();

  // Set initial selected workflow if eventID is provided
  if (props.eventID) {
    const selectedEvent = store.workflowEvents.find((e) => e.event_id === props.eventID);
    if (selectedEvent) {
      store.selectedItem = props.eventID;
      store.selectedWorkflow = selectedEvent;
      await store.loadWorkflowData(props.eventID);
    }
  }

  elRoot.value.closest('.is-loading')?.classList?.remove('is-loading');

  window.addEventListener('popstate', (e) => {
    if (e.state?.eventId) {
      const event = store.workflowEvents.find((ev) => ev.event_id === e.state.eventId);
      if (event) {
        selectWorkflowEvent(event);
      }
    }
  });
});
</script>

<template>
  <div ref="elRoot" class="workflow-container">
    <div class="workflow-sidebar">
      <div class="ui fluid vertical menu">
        <a
          v-for="event in store.workflowEvents"
          :key="event.event_id"
          class="item"
          :class="{ active: store.selectedItem === event.event_id }"
          :href="`${props.projectLink}/workflows/${event.event_id}`"
          @click.prevent="selectWorkflowEvent(event)"
        >
          <span class="workflow-status" :class="{ configured: isWorkflowConfigured(event) }">
            <span v-if="isWorkflowConfigured(event)" v-html="svg('octicon-dot-fill')" class="status-icon configured"></span>
            <span v-else class="status-icon unconfigured"></span>
          </span>
          {{ event.display_name }}
        </a>
      </div>
    </div>
    <div class="workflow-main">
      <div class="workflow-content">
        <div v-if="!store.selectedWorkflow" class="ui placeholder segment">
          <div class="ui icon header">
            <i class="settings icon"/>
            Select a workflow event to configure
          </div>
        </div>
        <div v-else class="workflow-editor">
          <div class="ui header">
            <i class="settings icon"/>
            {{ store.selectedWorkflow.display_name }}
          </div>
          <div class="workflow-form">
            <div class="ui form">
              <div class="field">
                <label>When</label>
                <div class="ui segment">
                  <div class="description">
                    This workflow will run when: <strong>{{ store.selectedWorkflow.display_name }}</strong>
                  </div>
                </div>
              </div>

              <div class="field">
                <label>Filters</label>
                <div class="ui segment">
                  <div class="field">
                    <label>Apply to</label>
                    <select class="ui dropdown" v-model="store.workflowFilters.scope">
                      <option value="">Issues And Pull Requests</option>
                      <option value="issue">Issues</option>
                      <option value="pull_request">Pull requests</option>
                    </select>
                  </div>
                </div>
              </div>

              <div class="field">
                <label>Actions</label>
                <div class="ui segment">
                  <div class="field">
                    <label>Move to column</label>
                    <select class="ui dropdown" v-model="store.workflowActions.column">
                      <option value="">Select column...</option>
                      <option v-for="column in store.projectColumns" :key="column.id" :value="column.id">
                        {{ column.title }}
                      </option>
                    </select>
                  </div>

                  <div class="field">
                    <div class="ui checkbox">
                      <input type="checkbox" v-model="store.workflowActions.closeIssue" id="close-issue">
                      <label for="close-issue">Close issue</label>
                    </div>
                  </div>
                </div>
              </div>

              <div class="actions">
                <button class="ui primary button" @click="saveWorkflow" :loading="store.saving">
                  Save workflow
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.workflow-container {
  display: flex;
  gap: 1rem;
  width: 100%;
  min-height: 500px;
}

.workflow-sidebar {
  width: 300px;
  flex-shrink: 0;
}

.workflow-main {
  flex: 1;
  min-width: 0;
}

.workflow-content {
  padding: 1rem;
}

.workflow-editor {
  margin-top: 1rem;
}

.workflow-form .field {
  margin-bottom: 1.5rem;
}

.workflow-form .field label {
  font-weight: bold;
  margin-bottom: 0.5rem;
  display: block;
}

.workflow-form .ui.segment {
  padding: 1rem;
  margin-bottom: 0.5rem;
}

.workflow-form .description {
  color: #666;
  font-style: italic;
}

.workflow-form .actions {
  margin-top: 2rem;
  padding-top: 1rem;
  border-top: 1px solid #ddd;
}

.ui.placeholder.segment {
  text-align: center;
  padding: 3rem;
}

.ui.vertical.menu .item.active {
  background-color: #f0f0f0;
  font-weight: bold;
}

.workflow-status {
  display: inline-flex;
  align-items: center;
  margin-right: 0.5rem;
}

.status-icon {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-right: 0.25rem;
}

.status-icon.configured {
  color: #28a745;
}

.status-icon.configured svg {
  width: 8px;
  height: 8px;
  fill: currentColor;
}

.status-icon.unconfigured {
  border: 1px solid #6c757d;
  border-radius: 50%;
  background-color: transparent;
}

@media (max-width: 768px) {
  .workflow-container {
    flex-direction: column;
  }

  .workflow-sidebar {
    width: 100%;
  }
}
</style>
