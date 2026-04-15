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

const locale = props.locale;
const {currentRun: run} = toRefs(props.store.viewData);

const runTriggeredAtIso = computed(() => {
  const t = props.store.viewData.currentRun.triggeredAt;
  return t ? new Date(t * 1000).toISOString() : '';
});

const currentAttempt = computed(() => {
  return run.value.attempts.find((attempt) => attempt.current);
});

const triggerUser = computed(() => {
  if (currentAttempt.value) {
    return {
      name: currentAttempt.value.triggerUserName,
      link: currentAttempt.value.triggerUserLink,
    };
  }
  return {
    name: run.value.commit.pusher.displayName,
    link: run.value.commit.pusher.link,
  };
});

const triggerPrefix = computed(() => {
  return run.value.runAttempt > 1
    ? locale.rerunTriggeredBy
    : locale.triggeredViaBy.replace('%s', run.value.triggerEvent);
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
        {{ triggerPrefix }}
        <a v-if="triggerUser.link" class="muted" :href="triggerUser.link">{{ triggerUser.name }}</a>
        <span v-else class="muted">{{ triggerUser.name }}</span>
        • <relative-time :datetime="runTriggeredAtIso" prefix=""/>
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
