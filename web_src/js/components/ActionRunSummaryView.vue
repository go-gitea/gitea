<script setup lang="ts">
import ActionRunStatus from './ActionRunStatus.vue';
import WorkflowGraph from './WorkflowGraph.vue';
import type {ActionsArtifact, ActionsRun, ActionsRunStatus} from '../modules/gitea-actions.ts';

defineOptions({
  name: 'ActionRunSummaryView',
});

defineProps<{
  run: ActionsRun;
  artifacts: ActionsArtifact[];
  locale: Record<string, any>;
  runTriggeredAtIso: string;
  runTriggerEventLabel: string;
}>();
</script>
<template>
  <div>
    <div class="action-run-summary-block">
      <p class="action-run-summary-trigger">
        {{ locale.triggeredVia.replace('%s', runTriggerEventLabel) }}
        &nbsp;•&nbsp;<relative-time :datetime="runTriggeredAtIso" prefix=" "/>
      </p>
      <p class="tw-mb-0">
        <ActionRunStatus :locale-status="locale.status[run.status]" :status="run.status" :size="16"/>
        <span class="tw-ml-2">{{ locale.status[run.status] }}</span>
        <span class="tw-ml-3">{{ locale.totalDuration }} {{ run.duration || '–' }}</span>
        <span class="tw-ml-3">{{ locale.artifactsTitle }}: {{ artifacts.length || 0 }}</span>
      </p>
    </div>
    <WorkflowGraph
      v-if="run.jobs.length > 0"
      :jobs="run.jobs"
      :run-link="run.link"
      :workflow-id="run.workflowID"
      class="workflow-graph-container"
    />
  </div>
</template>
<style scoped>
.action-run-summary-block {
  padding: 12px;
  margin-bottom: 12px;
  border-bottom: 1px solid var(--color-secondary);
}

.action-run-summary-trigger {
  margin-bottom: 6px;
  color: var(--color-text-light-2);
}
</style>
