<script setup lang="ts">
import ActionStatusIcon from './ActionStatusIcon.vue';
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

const isRerun = computed(() => run.value.runAttempt > 1);

// The summary's dependency graph is the workflow's top-level shape: a reusable caller
// renders as a single node, its expanded children belong to the caller's own detail page.
const topLevelJobs = computed(() => (run.value.jobs || []).filter((j) => !j.parentJobID));

const triggerUser = computed(() => {
  const currentAttempt = run.value.attempts.find((attempt) => attempt.current);
  if (currentAttempt) {
    return {name: currentAttempt.triggerUserName, link: currentAttempt.triggerUserLink};
  }
  const pusher = run.value.commit.pusher;
  return pusher.displayName ? {name: pusher.displayName, link: pusher.link} : null;
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
        <span>{{ isRerun ? locale.rerun : locale.triggeredVia.replace('%s', run.triggerEvent) }}</span>
        <template v-if="triggerUser">
          <span>•</span>
          <a v-if="triggerUser.link" class="muted" :href="triggerUser.link">{{ triggerUser.name }}</a>
          <span v-else class="muted">{{ triggerUser.name }}</span>
        </template>
        <span>•</span>
        <relative-time :datetime="run.triggeredAt || ''" prefix=""/>
      </div>
      <div class="flex-text-block">
        <ActionStatusIcon :locale-status="locale.status[run.status]" :status="run.status" :size="16" icon-variant="circle-fill"/>
        <span>{{ locale.status[run.status] }}</span> • <span>{{ locale.totalDuration }} {{ run.duration || '–' }}</span>
      </div>
    </div>
    <WorkflowGraph
      v-if="topLevelJobs.length > 0"
      :store="store"
      :jobs="topLevelJobs"
      :run-link="run.link"
      :workflow-id="run.workflowID"
      :locale="locale"
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
