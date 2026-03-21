<script lang="ts">
import {defineComponent, type PropType} from 'vue';
import ActionRunStatus from './ActionRunStatus.vue';
import WorkflowGraph from './WorkflowGraph.vue';

export default defineComponent({
  name: 'ActionRunSummaryView',
  components: {
    ActionRunStatus,
    WorkflowGraph,
  },
  props: {
    run: {
      type: Object as PropType<Record<string, any>>,
      required: true,
    },
    artifacts: {
      type: Array as PropType<Array<Record<string, any>>>,
      required: true,
    },
    locale: {
      type: Object as PropType<Record<string, any>>,
      required: true,
    },
    runTriggeredAtIso: {
      type: String,
      required: true,
    },
    runTriggerEventLabel: {
      type: String,
      required: true,
    },
  },
});
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
