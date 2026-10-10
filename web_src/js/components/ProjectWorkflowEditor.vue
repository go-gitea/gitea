<script lang="ts" setup>
import {computed} from 'vue';
import ProjectWorkflowLabelPicker from './ProjectWorkflowLabelPicker.vue';
import {confirmModal} from '../features/comp/ConfirmModal.ts';
import type {ProjectWorkflowLocale, ProjectWorkflowStore} from './ProjectWorkflowStore.ts';

const props = defineProps<{
  store: ProjectWorkflowStore;
  locale: ProjectWorkflowLocale;
}>();

const store = props.store;
const locale = props.locale;
const row = computed(() => store.selected!);

const hasFilter = (name: string) => row.value.event.filters.includes(name);
const hasAction = (name: string) => row.value.event.actions.includes(name);

const issueTypes = [
  {value: '', text: locale.issuesAndPullRequests},
  {value: 'issue', text: locale.issuesOnly},
  {value: 'pull_request', text: locale.pullRequestsOnly},
];
const issueStates = [
  {value: '', text: locale.noChange},
  {value: 'close', text: locale.closeIssue},
  {value: 'reopen', text: locale.reopenIssue},
];
const optionText = (options: Array<{value: string, text: string}>, value: string) => options.find((o) => o.value === value)!.text;

const toggleLabel = (ids: number[], id: number) => {
  const idx = ids.indexOf(id);
  if (idx === -1) ids.push(id);
  else ids.splice(idx, 1);
};

const onDelete = async () => {
  if (await confirmModal({content: locale.deleteConfirm, confirmButtonColor: 'red'})) await store.remove();
};
</script>

<template>
  <form class="ui form" :aria-label="row.displayName" @submit.prevent="store.save()">
    <h4 class="ui top attached header flex-left-right">
      <div class="flex-text-inline">
        {{ row.displayName }}
        <span v-if="row.workflow && !store.editing" class="ui basic label" :class="row.workflow.enabled ? 'green' : 'red'">
          {{ row.workflow.enabled ? locale.enabled : locale.disabled }}
        </span>
      </div>
      <div v-if="store.editing" class="flex-text-inline">
        <button v-if="row.workflow || row.key !== row.event.event" type="button" class="ui small button" :disabled="store.saving" @click="store.cancel()">{{ locale.cancel }}</button>
        <button type="submit" class="ui small primary button" :class="{loading: store.saving}" :disabled="store.saving">{{ locale.save }}</button>
        <button v-if="row.workflow" type="button" class="ui small red button" :disabled="store.saving" @click="onDelete">{{ locale.remove }}</button>
      </div>
      <div v-else-if="store.canWrite && row.workflow" class="flex-text-inline">
        <button type="button" class="ui small primary button" @click="store.editMode = true">{{ locale.edit }}</button>
        <button type="button" class="ui small button" :disabled="store.saving" @click="store.setEnabled(!row.workflow.enabled)">
          {{ row.workflow.enabled ? locale.disable : locale.enable }}
        </button>
        <button type="button" class="ui small button" :disabled="!store.canClone(row)" @click="store.clone()">{{ locale.clone }}</button>
      </div>
    </h4>
    <div class="ui attached segment">
      <template v-if="row.event.filters.length">
        <h5 class="ui dividing header">{{ locale.filters }}</h5>
        <div class="tw-pl-4">
          <div v-if="hasFilter('issue_type')" class="field">
            <label id="workflow-issue-type-label" for="workflow-issue-type">{{ locale.applyTo }}</label>
            <select v-if="store.editing" class="ui dropdown custom" id="workflow-issue-type" v-model="store.form.issue_type">
              <option v-for="o in issueTypes" :key="o.value" :value="o.value">{{ o.text }}</option>
            </select>
            <div v-else aria-labelledby="workflow-issue-type-label">{{ optionText(issueTypes, store.form.issue_type) }}</div>
          </div>
          <div v-if="hasFilter('source_column')" class="field">
            <label id="workflow-source-column-label" for="workflow-source-column">{{ locale.whenMovedFromColumn }}</label>
            <select v-if="store.editing" class="ui dropdown custom" id="workflow-source-column" v-model="store.form.source_column_id">
              <option :value="0">{{ locale.anyColumn }}</option>
              <option v-for="c in store.columns" :key="c.id" :value="c.id">{{ c.title }}</option>
            </select>
            <div v-else aria-labelledby="workflow-source-column-label">{{ store.columnTitle(store.form.source_column_id) ?? locale.anyColumn }}</div>
          </div>
          <div v-if="hasFilter('target_column')" class="field">
            <label id="workflow-target-column-label" for="workflow-target-column">{{ locale.whenMovedToColumn }}</label>
            <select v-if="store.editing" class="ui dropdown custom" id="workflow-target-column" v-model="store.form.target_column_id">
              <option :value="0">{{ locale.anyColumn }}</option>
              <option v-for="c in store.columns" :key="c.id" :value="c.id">{{ c.title }}</option>
            </select>
            <div v-else aria-labelledby="workflow-target-column-label">{{ store.columnTitle(store.form.target_column_id) ?? locale.anyColumn }}</div>
          </div>
          <div v-if="hasFilter('labels')" class="field">
            <label>{{ locale.onlyIfHasLabels }}</label>
            <ProjectWorkflowLabelPicker
              :labels="store.labels" :selected-ids="store.form.label_ids" :readonly="!store.editing"
              :placeholder="locale.anyLabel" :field-label="locale.onlyIfHasLabels"
              @toggle="toggleLabel(store.form.label_ids, $event)"
            />
          </div>
        </div>
      </template>

      <h5 class="ui dividing header">{{ locale.actions }}</h5>
      <div class="tw-pl-4">
        <div v-if="hasAction('column')" class="field">
          <label id="workflow-column-label" for="workflow-column">{{ locale.moveToColumn }}</label>
          <select v-if="store.editing" class="ui dropdown custom" id="workflow-column" v-model="store.form.column_id">
            <option :value="0">{{ locale.selectColumn }}</option>
            <option v-for="c in store.columns" :key="c.id" :value="c.id">{{ c.title }}</option>
          </select>
          <div v-else aria-labelledby="workflow-column-label">{{ store.columnTitle(store.form.column_id) ?? locale.none }}</div>
        </div>
        <div v-if="hasAction('add_labels')" class="field">
          <label>{{ locale.addLabels }}</label>
          <ProjectWorkflowLabelPicker
            :labels="store.labels" :selected-ids="store.form.add_label_ids" :readonly="!store.editing"
            :placeholder="locale.none" :field-label="locale.addLabels"
            @toggle="toggleLabel(store.form.add_label_ids, $event)"
          />
        </div>
        <div v-if="hasAction('remove_labels')" class="field">
          <label>{{ locale.removeLabels }}</label>
          <ProjectWorkflowLabelPicker
            :labels="store.labels" :selected-ids="store.form.remove_label_ids" :readonly="!store.editing"
            :placeholder="locale.none" :field-label="locale.removeLabels"
            @toggle="toggleLabel(store.form.remove_label_ids, $event)"
          />
        </div>
        <div v-if="hasAction('issue_state')" class="field">
          <label id="workflow-issue-state-label" for="workflow-issue-state">{{ locale.issueState }}</label>
          <select v-if="store.editing" class="ui dropdown custom" id="workflow-issue-state" v-model="store.form.issue_state">
            <option v-for="o in issueStates" :key="o.value" :value="o.value">{{ o.text }}</option>
          </select>
          <div v-else aria-labelledby="workflow-issue-state-label">{{ optionText(issueStates, store.form.issue_state) }}</div>
        </div>
      </div>
    </div>
  </form>
</template>
