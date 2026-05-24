<script setup lang="ts">
import WorkflowGraph from './WorkflowGraph.vue';
import type {ActionRunViewStore} from "./ActionRunView.ts";
import {computed, onBeforeUnmount, onMounted, toRefs} from "vue";

defineOptions({
  name: 'ActionRunSummaryView',
});

const props = defineProps<{
  store: ActionRunViewStore;
  locale: Record<string, any>;
  artifactCount: number;
}>();

const locale = props.locale;
const {currentRun: run} = toRefs(props.store.viewData);

const isRerun = computed(() => run.value.runAttempt > 1);

const triggerUser = computed(() => {
  const currentAttempt = run.value.attempts.find((attempt) => attempt.current);
  if (currentAttempt) {
    return {
      name: currentAttempt.triggerUserName,
      link: currentAttempt.triggerUserLink,
      avatar: currentAttempt.triggerUserAvatar,
    };
  }
  const pusher = run.value.commit.pusher;
  return pusher.displayName ? {
    name: pusher.displayName,
    link: pusher.link,
    avatar: pusher.avatarLink,
  } : null;
});

const triggerLabel = computed(() => {
  if (isRerun.value) return locale.rerunTriggered;
  return locale.triggeredVia.replace('%s', run.value.triggerEvent);
});

const artifactsDisplay = computed(() => props.artifactCount > 0 ? String(props.artifactCount) : '–');

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
      <div class="action-run-summary-trigger">
        <span class="action-run-summary-label">
          {{ triggerLabel }} <relative-time :datetime="run.triggeredAt || ''" prefix=""/>
        </span>
        <div class="flex-text-block tw-flex-wrap action-run-summary-trigger-content">
          <template v-if="triggerUser">
            <a v-if="triggerUser.link" class="flex-text-inline action-run-summary-user silenced" :href="triggerUser.link">
              <img
                v-if="triggerUser.avatar"
                class="ui avatar tw-align-middle"
                :src="triggerUser.avatar"
                width="16"
                height="16"
                :alt="triggerUser.name"
              >
              <span>{{ triggerUser.name }}</span>
            </a>
            <span v-else class="flex-text-inline action-run-summary-user">
              <img
                v-if="triggerUser.avatar"
                class="ui avatar tw-align-middle"
                :src="triggerUser.avatar"
                width="16"
                height="16"
                :alt="triggerUser.name"
              >
              <span>{{ triggerUser.name }}</span>
            </span>
          </template>
          <a v-if="run.pullRequest" class="action-run-summary-pr silenced" :href="run.pullRequest.link">{{ run.pullRequest.index }}</a>
          <span v-else-if="run.commit.branch.name" class="action-run-summary-branch-label tw-max-w-full">
            <a
              v-if="!run.commit.branch.isDeleted && run.commit.branch.link"
              class="gt-ellipsis silenced"
              :href="run.commit.branch.link"
              :title="run.commit.branch.name"
            >{{ run.commit.branch.name }}</a>
            <span
              v-else
              class="gt-ellipsis tw-line-through"
              :title="run.commit.branch.name"
            >{{ run.commit.branch.name }}</span>
          </span>
        </div>
      </div>

      <div class="action-run-summary-stat-divider"/>

      <div class="action-run-summary-stat">
        <span class="action-run-summary-label">{{ locale.statusLabel }}</span>
        <span class="action-run-summary-stat-value">{{ locale.status[run.status] }}</span>
      </div>

      <div class="action-run-summary-stat">
        <span class="action-run-summary-label">{{ locale.totalDuration }}</span>
        <span class="action-run-summary-stat-value">{{ run.duration || '–' }}</span>
      </div>

      <div class="action-run-summary-stat action-run-summary-stat-last">
        <span class="action-run-summary-label">{{ locale.artifactsTitle }}</span>
        <span class="action-run-summary-stat-value">{{ artifactsDisplay }}</span>
      </div>
    </div>
    <WorkflowGraph
      v-if="run.jobs.length > 0"
      :store="store"
      :jobs="run.jobs"
      :run-link="run.link"
      :workflow-id="run.workflowID"
      :workflow-link="`${run.link}/workflow`"
      :trigger-event="run.triggerEvent"
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
  flex-wrap: wrap;
  align-items: flex-end;
  padding: 12px 16px;
  border-bottom: 1px solid var(--color-secondary);
  background: var(--color-console-bg);
}

.action-run-summary-trigger {
  flex: 0 1 auto;
  min-width: 0;
  max-width: 100%;
  margin-right: 24px;
}

.action-run-summary-label {
  display: block;
  margin-bottom: 4px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--color-text-light-2);
}

.action-run-summary-trigger-content {
  color: var(--color-text-light-2);
  align-items: center;
}

.action-run-summary-user {
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
  line-height: 16px;
}

.action-run-summary-user .ui.avatar {
  margin: 0;
}

.action-run-summary-pr {
  color: var(--color-text);
  line-height: 16px;
}

.action-run-summary-branch-label {
  display: inline-flex;
  align-items: center;
  max-width: 200px;
  min-height: 20px;
  padding: 0 6px;
  border-radius: var(--border-radius);
  background: var(--color-primary-light-6);
  color: var(--color-primary);
  font-size: 12px;
  line-height: 20px;
  font-family: var(--fonts-monospace);
}

.action-run-summary-branch-label a {
  color: inherit;
}

.action-run-summary-branch-label a:hover {
  text-decoration: underline;
}

.action-run-summary-user:hover span {
  color: var(--color-primary);
}

.action-run-summary-stat {
  flex: 0 0 auto;
  min-width: 72px;
  margin-left: 24px;
  margin-right: 24px;
}

.action-run-summary-stat-last {
  margin-right: 0;
}

.action-run-summary-stat-divider {
  display: none;
  flex: 0 0 100%;
  margin: 8px 0;
  border-bottom: 1px solid var(--color-secondary);
}

.action-run-summary-stat-value {
  display: block;
  font-size: 16px;
  line-height: 1.25;
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
}

@media (max-width: 767.98px) {
  .action-run-summary-block {
    align-items: flex-start;
  }

  .action-run-summary-trigger {
    flex: 0 0 100%;
    margin-right: 0;
  }

  .action-run-summary-stat {
    margin-left: 0;
    margin-right: 24px;
  }

  .action-run-summary-stat-divider {
    display: block;
  }
}
</style>
