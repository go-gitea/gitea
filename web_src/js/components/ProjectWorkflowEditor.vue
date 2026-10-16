<script lang="ts" setup>
import {computed} from 'vue';
import ProjectWorkflowDropdown from './ProjectWorkflowDropdown.vue';
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
const columnOptions = (emptyText: string) => computed(() => [{value: 0, text: emptyText}, ...store.columns.map((c) => ({value: c.id, text: c.title}))]);
const anyColumnOptions = columnOptions(locale.anyColumn);
const noColumnOptions = columnOptions(locale.none);
const labelOptions = computed(() => store.labels.map((l) => ({value: l.id, text: l.name, html: l.html})));

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
        <button v-if="row.workflow || row.key !== row.event.event" type="button" class="ui tiny button" :disabled="store.saving" @click="store.cancel()">{{ locale.cancel }}</button>
        <button type="submit" class="ui tiny primary button" :class="{loading: store.saving}" :disabled="store.saving">{{ locale.save }}</button>
        <button v-if="row.workflow" type="button" class="ui tiny red button" :disabled="store.saving" @click="onDelete">{{ locale.remove }}</button>
      </div>
      <div v-else-if="store.canWrite && row.workflow" class="flex-text-inline">
        <button type="button" class="ui tiny primary button" @click="store.editMode = true">{{ locale.edit }}</button>
        <button type="button" class="ui tiny button" :disabled="store.saving" @click="store.setEnabled(!row.workflow.enabled)">
          {{ row.workflow.enabled ? locale.disable : locale.enable }}
        </button>
        <button type="button" class="ui tiny button" :disabled="!store.canClone(row)" @click="store.clone()">{{ locale.clone }}</button>
      </div>
    </h4>
    <div class="ui attached segment">
      <template v-if="row.event.filters.length">
        <h5 class="ui dividing header">{{ locale.filters }}</h5>
        <ProjectWorkflowDropdown
          v-if="hasFilter('issue_type')" v-model="store.form.issue_type"
          :label="locale.applyTo" :options="issueTypes" :readonly="!store.editing"
        />
        <ProjectWorkflowDropdown
          v-if="hasFilter('source_column')" v-model="store.form.source_column_id"
          :label="locale.whenMovedFromColumn" :options="anyColumnOptions" :readonly="!store.editing"
        />
        <ProjectWorkflowDropdown
          v-if="hasFilter('target_column')" v-model="store.form.target_column_id"
          :label="locale.whenMovedToColumn" :options="anyColumnOptions" :readonly="!store.editing"
        />
        <ProjectWorkflowDropdown
          v-if="hasFilter('labels')" v-model="store.form.label_ids"
          :label="locale.onlyIfHasLabels" :options="labelOptions" :readonly="!store.editing"
          :placeholder="locale.anyLabel" :filter-text="locale.filterLabels" :clear-text="locale.clearLabels"
        />
      </template>

      <h5 class="ui dividing header">{{ locale.actions }}</h5>
      <ProjectWorkflowDropdown
        v-if="hasAction('column')" v-model="store.form.column_id"
        :label="locale.moveToColumn" :options="noColumnOptions" :readonly="!store.editing"
      />
      <ProjectWorkflowDropdown
        v-if="hasAction('add_labels')" v-model="store.form.add_label_ids"
        :label="locale.addLabels" :options="labelOptions" :readonly="!store.editing"
        :placeholder="locale.none" :filter-text="locale.filterLabels" :clear-text="locale.clearLabels"
      />
      <ProjectWorkflowDropdown
        v-if="hasAction('remove_labels')" v-model="store.form.remove_label_ids"
        :label="locale.removeLabels" :options="labelOptions" :readonly="!store.editing"
        :placeholder="locale.none" :filter-text="locale.filterLabels" :clear-text="locale.clearLabels"
      />
      <ProjectWorkflowDropdown
        v-if="hasAction('issue_state')" v-model="store.form.issue_state"
        :label="locale.issueState" :options="issueStates" :readonly="!store.editing"
      />
    </div>
  </form>
</template>
