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
      <div class="flex-text-block">
        {{ locale.triggeredVia.replace('%s', run.triggerEvent) }} • <relative-time :datetime="runTriggeredAtIso" prefix=""/>
      </div>
      <div class="flex-text-block">
        <ActionRunStatus :locale-status="locale.status[run.status]" :status="run.status" :size="16"/>
        <span>{{ locale.status[run.status] }}</span> • <span>{{ locale.totalDuration }} {{ run.duration || '–' }}</span>
      </div>
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
  color: var(--color-text-light-1);
}

.action-run-summary-block {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px;
  border-bottom: 1px solid var(--color-secondary);
  border-radius: var(--border-radius) var(--border-radius) 0 0;
  background: var(--color-box-header);
}
</style>
