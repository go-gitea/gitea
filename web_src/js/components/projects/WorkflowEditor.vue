<script lang="ts" setup>
import {inject} from 'vue';
import WorkflowLabelPicker from './WorkflowLabelPicker.vue';
import type {WorkflowStoreState, WorkflowEvent} from './WorkflowStore.ts';

const store = inject<WorkflowStoreState>('workflowStore')!;

const props = defineProps<{
  locale: {
    when: string;
    runWhen: string;
    filters: string;
    applyTo: string;
    whenMovedFromColumn: string;
    whenMovedToColumn: string;
    onlyIfHasLabels: string;
    anyColumn: string;
    anyLabel: string;
    issuesAndPullRequests: string;
    issuesOnly: string;
    pullRequestsOnly: string;
    actions: string;
    moveToColumn: string;
    selectColumn: string;
    addLabels: string;
    removeLabels: string;
    none: string;
    issueState: string;
    noChange: string;
    closeIssue: string;
    reopenIssue: string;
    viewWorkflowConfiguration: string;
    configureWorkflow: string;
    cancel: string;
    save: string;
    delete: string;
    edit: string;
    disable: string;
    enable: string;
    disabled: string;
    enabled: string;
    clone: string;
    cloneTooltip: string;
  };
  isInEditMode: boolean;
  showCancelButton: boolean;
  canCloneSelectedWorkflow: boolean;
}>(); 

const emit = defineEmits<{
  'toggle-edit-mode': [];
  'save-workflow': [];
  'delete-workflow': [];
  'toggle-workflow-status': [];
  'clone-workflow': [sourceWorkflow: WorkflowEvent];
}>();

// Whether the given filter type is available for the selected workflow.
const hasFilter = (type: string) =>
  store.selectedWorkflow?.capabilities?.available_filters?.includes(type) ?? false;

// Whether the given action type is available for the selected workflow.
const hasAction = (type: string) =>
  store.selectedWorkflow?.capabilities?.available_actions?.includes(type) ?? false;

const hasAvailableFilters = () =>
  (store.selectedWorkflow?.capabilities?.available_filters?.length ?? 0) > 0;

const columnTitle = (id: string, fallback: string) =>
  store.projectColumns.find((c: {id: number; title: string}) => String(c.id) === id)?.title ?? fallback;

// Toggle a label in filter_labels, add_labels, or remove_labels.
const toggleLabel = (type: string, labelId: string) => {
  const map: Record<string, string[]> = {
    filter_labels: store.workflowFilters.labels,
    add_labels: store.workflowActions.add_labels,
    remove_labels: store.workflowActions.remove_labels,
  };
  const list = map[type];
  if (!list) return;
  const idx = list.indexOf(labelId);
  if (idx > -1) list.splice(idx, 1);
  else list.push(labelId);
};
</script>

<template>
  <div class="workflow-main">
    <!-- No workflow selected yet -->
    <div v-if="!store.selectedWorkflow" class="workflow-placeholder">
      <div class="placeholder-content">
        <div class="placeholder-icon"><i class="huge settings icon"/></div>
      </div>
    </div>

    <!-- Workflow editor / viewer -->
    <div v-else class="workflow-editor">
      <!-- ── Header ─────────────────────────────────────────────── -->
      <div class="editor-header">
        <div class="editor-title">
          <h2>
            <i class="settings icon"/>
            {{ store.selectedWorkflow.display_name }}
            <span
              v-if="store.selectedWorkflow.id > 0 && !isInEditMode"
              class="workflow-status"
              :class="store.selectedWorkflow.enabled ? 'status-enabled' : 'status-disabled'"
            >
              {{ store.selectedWorkflow.enabled ? locale.enabled : locale.disabled }}
            </span>
          </h2>
          <p v-if="!store.selectedWorkflow.id || isInEditMode">{{ locale.configureWorkflow }}</p>
          <p v-else>{{ locale.viewWorkflowConfiguration }}</p>
        </div>

        <div class="editor-actions-header">
          <!-- Edit-mode buttons -->
          <template v-if="isInEditMode">
            <button v-if="showCancelButton" class="ui small button" @click="emit('toggle-edit-mode')">
              {{ locale.cancel }}
            </button>
            <button class="ui small primary button" :disabled="store.saving" @click="emit('save-workflow')">
              {{ locale.save }}
            </button>
            <button v-if="store.selectedWorkflow.id > 0" class="ui small red button" @click="emit('delete-workflow')">
              {{ locale.delete }}
            </button>
          </template>

          <!-- View-mode buttons (saved workflows only) -->
          <template v-else-if="store.selectedWorkflow.id > 0">
            <button class="ui small primary button" @click="emit('toggle-edit-mode')">{{ locale.edit }}</button>
            <button
              class="ui small button"
              :class="store.selectedWorkflow.enabled ? 'red' : 'green'"
              @click="emit('toggle-workflow-status')"
            >
              {{ store.selectedWorkflow.enabled ? locale.disable : locale.enable }}
            </button>
            <button
              class="ui small button"
              :disabled="!canCloneSelectedWorkflow"
              :title="locale.cloneTooltip"
              @click="emit('clone-workflow', store.selectedWorkflow)"
            >
              {{ locale.clone }}
            </button>
          </template>
        </div>
      </div>

      <!-- ── Form ──────────────────────────────────────────────── -->
      <div class="editor-content">
        <div class="form" :class="{ readonly: !isInEditMode }">
          <!-- When -->
          <div class="field">
            <label>{{ locale.when }}</label>
            <div class="segment">
              <div class="description">
                {{ locale.runWhen }}<strong>{{ store.selectedWorkflow.display_name }}</strong>
              </div>
            </div>
          </div>

          <!-- Filters -->
          <div v-if="hasAvailableFilters()" class="field">
            <label>{{ locale.filters }}</label>
            <div class="segment">
              <!-- Apply to (issue type) -->
              <div v-if="hasFilter('issue_type')" class="field">
                <label>{{ locale.applyTo }}</label>
                <select v-if="isInEditMode" class="column-select" v-model="store.workflowFilters.issue_type">
                  <option value="">{{ locale.issuesAndPullRequests }}</option>
                  <option value="issue">{{ locale.issuesOnly }}</option>
                  <option value="pull_request">{{ locale.pullRequestsOnly }}</option>
                </select>
                <div v-else class="readonly-value">
                  {{ store.workflowFilters.issue_type === 'issue' ? locale.issuesOnly :
                    store.workflowFilters.issue_type === 'pull_request' ? locale.pullRequestsOnly :
                    locale.issuesAndPullRequests }}
                </div>
              </div>

              <!-- Source column -->
              <div v-if="hasFilter('source_column')" class="field">
                <label>{{ locale.whenMovedFromColumn }}</label>
                <select v-if="isInEditMode" v-model="store.workflowFilters.source_column" class="column-select">
                  <option value="">{{ locale.anyColumn }}</option>
                  <option v-for="col in store.projectColumns" :key="col.id" :value="String(col.id)">{{ col.title }}</option>
                </select>
                <div v-else class="readonly-value">{{ columnTitle(store.workflowFilters.source_column, locale.anyColumn) }}</div>
              </div>

              <!-- Target column -->
              <div v-if="hasFilter('target_column')" class="field">
                <label>{{ locale.whenMovedToColumn }}</label>
                <select v-if="isInEditMode" v-model="store.workflowFilters.target_column" class="column-select">
                  <option value="">{{ locale.anyColumn }}</option>
                  <option v-for="col in store.projectColumns" :key="col.id" :value="String(col.id)">{{ col.title }}</option>
                </select>
                <div v-else class="readonly-value">{{ columnTitle(store.workflowFilters.target_column, locale.anyColumn) }}</div>
              </div>

              <!-- Filter labels -->
              <div v-if="hasFilter('labels')" class="field">
                <label>{{ locale.onlyIfHasLabels }}</label>
                <WorkflowLabelPicker
                  :labels="store.projectLabels"
                  :selected-ids="store.workflowFilters.labels"
                  :placeholder="locale.anyLabel"
                  :readonly="!isInEditMode"
                  @toggle="id => toggleLabel('filter_labels', id)"
                />
              </div>
            </div>
          </div>

          <!-- Actions -->
          <div class="field">
            <label>{{ locale.actions }}</label>
            <div class="segment">
              <!-- Move to column -->
              <div v-if="hasAction('column')" class="field">
                <label>{{ locale.moveToColumn }}</label>
                <select v-if="isInEditMode" v-model="store.workflowActions.column" class="column-select">
                  <option value="">{{ locale.selectColumn }}</option>
                  <option v-for="col in store.projectColumns" :key="col.id" :value="String(col.id)">{{ col.title }}</option>
                </select>
                <div v-else class="readonly-value">{{ columnTitle(store.workflowActions.column, locale.none) }}</div>
              </div>

              <!-- Add labels -->
              <div v-if="hasAction('add_labels')" class="field">
                <label>{{ locale.addLabels }}</label>
                <WorkflowLabelPicker
                  :labels="store.projectLabels"
                  :selected-ids="store.workflowActions.add_labels"
                  :placeholder="locale.none"
                  :readonly="!isInEditMode"
                  @toggle="id => toggleLabel('add_labels', id)"
                />
              </div>

              <!-- Remove labels -->
              <div v-if="hasAction('remove_labels')" class="field">
                <label>{{ locale.removeLabels }}</label>
                <WorkflowLabelPicker
                  :labels="store.projectLabels"
                  :selected-ids="store.workflowActions.remove_labels"
                  :placeholder="locale.none"
                  :readonly="!isInEditMode"
                  @toggle="id => toggleLabel('remove_labels', id)"
                />
              </div>

              <!-- Issue state -->
              <div v-if="hasAction('issue_state')" class="field">
                <label for="issue-state-action">{{ locale.issueState }}</label>
                <select
                  v-if="isInEditMode"
                  id="issue-state-action"
                  class="column-select"
                  v-model="store.workflowActions.issue_state"
                >
                  <option value="">{{ locale.noChange }}</option>
                  <option value="close">{{ locale.closeIssue }}</option>
                  <option value="reopen">{{ locale.reopenIssue }}</option>
                </select>
                <div v-else class="readonly-value">
                  {{ store.workflowActions.issue_state === 'close' ? locale.closeIssue :
                    store.workflowActions.issue_state === 'reopen' ? locale.reopenIssue : locale.noChange }}
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
.workflow-main {
  flex: 1;
  background: var(--color-body);
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.workflow-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-light-2);
}

.placeholder-icon { font-size: 4rem; opacity: 0.3; }

.workflow-editor {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.editor-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid var(--color-secondary);
  background: var(--color-box-header);
  flex-shrink: 0;
}

.editor-title h2 {
  margin: 0 0 0.25rem;
  font-size: 1.2rem;
  font-weight: 600;
  color: var(--color-text);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.editor-title p {
  margin: 0;
  color: var(--color-text-light-2);
  font-size: 0.875rem;
}

.editor-actions-header {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-shrink: 0;
}

.editor-content {
  flex: 1;
  padding: 1.5rem;
  overflow-y: auto;
}

/* Status badge */
.workflow-status {
  display: inline-flex;
  align-items: center;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
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

/* Form -------------------------------------------------------------- */
.form .field { margin-bottom: 1rem; }
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

.description { color: var(--color-text-light-2); }

.form.readonly { pointer-events: none; }
</style>
