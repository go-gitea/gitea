<script setup lang="ts">
import ActionRunStatus from './ActionRunStatus.vue';
import WorkflowGraph from './WorkflowGraph.vue';
import type {ActionRunViewStore} from "./ActionRunView.ts";
import {computed, onBeforeUnmount, onMounted, toRefs} from "vue";

defineOptions({
  name: 'ActionRunSummaryView',
});

const props = defineProps<{
  store: ActionRunViewStore;
  locale: Record<string, any>;
}>();

const {currentRun: run} = toRefs(props.store.viewData);

const runTriggeredAtIso = computed(() => {
  const t = props.store.viewData.currentRun.triggeredAt;
  return t ? new Date(t * 1000).toISOString() : '';
});

onMounted(async () => {
  await props.store.startPollingCurrentRun();
});

onBeforeUnmount(() => {
  props.store.stopPollingCurrentRun();
});
</script>
<template>
  <div class="action-run-summary-view">
    <div class="action-run-summary-block">
      <p class="action-run-summary-trigger">
        {{ locale.triggeredVia.replace('%s', run.triggerEvent) }}&nbsp;•&nbsp;<relative-time :datetime="runTriggeredAtIso" prefix=""/>
      </p>
      <p class="tw-mb-0">
        <ActionRunStatus :locale-status="locale.status[run.status]" :status="run.status" :size="16"/>
        <span class="tw-ml-2">{{ locale.status[run.status] }}</span>&nbsp;•&nbsp;<span>{{ locale.totalDuration }} {{ run.duration || '–' }}</span>
      </p>
    </div>
    <WorkflowGraph
      v-if="run.jobs.length > 0"
      :store="store"
      :jobs="run.jobs"
      :run-link="run.link"
      :workflow-id="run.workflowID"
    />
  </div>
</template>
<style scoped>
.action-run-summary-view {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.action-run-summary-block {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 6px;
  padding: 12px;
  border-bottom: 1px solid var(--color-secondary);
}

.action-run-summary-trigger {
  margin-bottom: 0;
  color: var(--color-text-light-2);
}

@media (max-width: 767.98px) {
  .action-run-summary-block {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
