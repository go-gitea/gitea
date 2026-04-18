<script setup lang="ts">
import {SvgIcon} from '../svg.ts';
import ActionRunStatus from './ActionRunStatus.vue';
import {computed, toRefs} from 'vue';
import {POST, DELETE} from '../modules/fetch.ts';
import ActionRunSummaryView from './ActionRunSummaryView.vue';
import ActionRunJobView from './ActionRunJobView.vue';
import {createActionRunViewStore} from "./ActionRunView.ts";
import type {ActionsRunAttempt} from '../modules/gitea-actions.ts';

defineOptions({
  name: 'RepoActionView',
});

const props = defineProps<{
  jobId: number;
  viewUrl: string;
  locale: Record<string, any>;
}>();

const locale = props.locale;
const store = createActionRunViewStore(props.viewUrl);
const {currentRun: run , runArtifacts: artifacts} = toRefs(store.viewData);

function formatAttemptTitle(attempt: ActionsRunAttempt) {
  return attempt.latest ? `${locale.latestAttempt} #${attempt.attempt}` : `${locale.attempt} #${attempt.attempt}`;
}

function formatCurrentAttemptTitle(attempt: ActionsRunAttempt) {
  return attempt.latest ? `${locale.latest} #${attempt.attempt}` : formatAttemptTitle(attempt);
}

const artifactActionSuffix = computed(() => run.value.runAttempt > 0 ? `?attempt=${run.value.runAttempt}` : '');

function cancelRun() {
  POST(`${run.value.link}/cancel`);
}

function approveRun() {
  POST(`${run.value.link}/approve`);
}

async function deleteArtifact(name: string) {
  if (!window.confirm(locale.confirmDeleteArtifact.replace('%s', name))) return;
  await DELETE(`${run.value.link}/artifacts/${encodeURIComponent(name)}${artifactActionSuffix.value}`);
  await store.forceReloadCurrentRun();
}
</script>
<template>
  <!-- make the view container full width to make users easier to read logs -->
  <div class="ui fluid container">
    <div class="action-view-header">
      <div class="action-info-summary">
        <div class="action-info-summary-title">
          <ActionRunStatus :locale-status="locale.status[run.status]" :status="run.status" :size="20"/>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <h2 class="action-info-summary-title-text" v-html="run.titleHTML"/>
        </div>
        <div class="flex-text-block tw-shrink-0 tw-flex-wrap">
          <button class="ui basic small compact button primary" @click="approveRun()" v-if="run.canApprove">
            {{ locale.approve }}
          </button>
          <button class="ui basic small compact button red" @click="cancelRun()" v-else-if="run.canCancel">
            {{ locale.cancel }}
          </button>
          <template v-if="run.canRerun">
            <div v-if="run.canRerunFailed" class="ui small compact buttons">
              <button class="ui basic small compact button link-action" :data-url="`${run.link}/rerun-failed`">
                {{ locale.rerun_failed }}
              </button>
              <div class="ui basic small compact dropdown icon button">
                <SvgIcon name="octicon-triangle-down" :size="14"/>
                <div class="menu">
                  <div class="item link-action" :data-url="`${run.link}/rerun`">
                    {{ locale.rerun_all }}
                  </div>
                </div>
              </div>
            </div>
            <button v-else class="ui basic small compact button link-action" :data-url="`${run.link}/rerun`">
              {{ locale.rerun_all }}
            </button>
          </template>
          <div v-if="run.attempts.length > 1" class="ui dropdown jump basic small compact button attempt-switcher tw-relative">
            <SvgIcon name="octicon-history" :size="14" class="tw-mr-1.5"/>
            <span class="text tw-mr-1.5">{{ formatCurrentAttemptTitle(run.attempts.find((attempt) => attempt.current)!) }}</span>
            <SvgIcon name="octicon-triangle-down" :size="14" class="dropdown icon"/>
            <div class="menu attempt-switcher-menu">
              <a
                v-for="attempt in run.attempts"
                :key="attempt.attempt"
                class="item attempt-switcher-item"
                :class="attempt.current ? 'selected' : ''"
                :href="attempt.link"
              >
                <div class="tw-flex tw-items-start tw-gap-3">
                  <div class="tw-flex tw-justify-center tw-w-4 tw-flex-shrink-0 tw-pt-[3px]">
                    <SvgIcon v-if="attempt.current" name="octicon-check" :size="14"/>
                  </div>
                  <div class="tw-flex tw-flex-col tw-flex-1 tw-min-w-0 tw-gap-1">
                    <div class="tw-whitespace-nowrap tw-text-sm tw-font-semibold">
                      <span>{{ formatAttemptTitle(attempt) }}</span>
                    </div>
                    <div class="attempt-switcher-item-meta">
                      <span class="tw-inline-flex tw-items-center tw-gap-1 tw-flex-shrink-0">
                        <ActionRunStatus :locale-status="locale.status[attempt.status]" :status="attempt.status" :size="14"/>
                        <span>{{ locale.status[attempt.status] }}</span>
                      </span>
                      <span class="tw-min-w-0 gt-ellipsis">
                        <relative-time :datetime="new Date(attempt.triggeredAt * 1000).toISOString()" prefix=""/>
                        {{ locale.attemptTriggeredBy.replace('%s', attempt.triggerUserName) }}
                      </span>
                    </div>
                  </div>
                </div>
              </a>
            </div>
          </div>
        </div>
      </div>
      <div class="action-commit-summary">
        <span>
          <a v-if="run.workflowLink" class="muted" :href="run.workflowLink"><b>{{ run.workflowID }}</b></a>
          <b v-else>{{ run.workflowID }}</b>
          :
        </span>
        <template v-if="run.isSchedule">
          {{ locale.scheduled }}
        </template>
        <template v-else>
          {{ locale.commit }}
          <a class="muted" :href="run.commit.link">{{ run.commit.shortSHA }}</a>
          {{ locale.pushedBy }}
          <a class="muted" :href="run.commit.pusher.link">{{ run.commit.pusher.displayName }}</a>
        </template>
        <span class="ui label tw-max-w-full" v-if="run.commit.shortSHA">
          <span v-if="run.commit.branch.isDeleted" class="gt-ellipsis tw-line-through" :data-tooltip-content="run.commit.branch.name">{{ run.commit.branch.name }}</span>
          <a v-else class="gt-ellipsis" :href="run.commit.branch.link" :data-tooltip-content="run.commit.branch.name">{{ run.commit.branch.name }}</a>
        </span>
      </div>
    </div>
    <div class="action-view-body">
      <div class="action-view-left">
        <!-- summary -->
        <a class="job-brief-item silenced" :href="run.viewLink" :class="!props.jobId ? 'selected' : ''">
          <SvgIcon name="octicon-list-unordered"/>
          <span class="gt-ellipsis">{{ locale.summary }}</span>
        </a>

        <!-- jobs list -->
        <div class="ui divider"/>
        <div class="left-list-header">{{ locale.allJobs }}</div>
        <!-- unlike other lists, the items have paddings already -->
        <ul class="ui relaxed list flex-items-block tw-p-0">
          <li class="item job-brief-item" v-for="job in run.jobs" :key="job.id" :class="props.jobId === job.id ? 'selected' : ''">
            <a class="tw-contents silenced" :href="job.link">
              <ActionRunStatus :locale-status="locale.status[job.status]" :status="job.status"/>
              <span class="tw-flex-1 gt-ellipsis">{{ job.name }}</span>
              <SvgIcon name="octicon-sync" role="button" :data-tooltip-content="locale.rerun" class="tw-cursor-pointer link-action interact-fg" :data-url="`${run.link}/jobs/${job.id}/rerun`" v-if="job.canRerun"/>
              <span>{{ job.duration }}</span>
            </a>
          </li>
        </ul>

        <!-- artifacts list -->
        <template v-if="artifacts.length > 0">
          <div class="ui divider"/>
          <div class="left-list-header">{{ locale.artifactsTitle }} ({{ artifacts.length }})</div>
          <ul class="ui relaxed list flex-items-block">
            <li class="item" v-for="artifact in artifacts" :key="artifact.name">
              <template v-if="artifact.status !== 'expired'">
                <a class="tw-flex-1 flex-text-block" target="_blank" :href="`${run.link}/artifacts/${artifact.name}${artifactActionSuffix}`">
                  <SvgIcon name="octicon-file" class="tw-text-text"/>
                  <span class="tw-flex-1 gt-ellipsis">{{ artifact.name }}</span>
                </a>
                <a v-if="run.canDeleteArtifact" @click="deleteArtifact(artifact.name)">
                  <SvgIcon name="octicon-trash" class="tw-text-text"/>
                </a>
              </template>
              <span v-else class="flex-text-block tw-flex-1 tw-text-grey-light">
                <SvgIcon name="octicon-file"/>
                <span class="tw-flex-1 gt-ellipsis">{{ artifact.name }}</span>
                <span class="ui label tw-text-grey-light tw-flex-shrink-0">{{ locale.artifactExpired }}</span>
              </span>
            </li>
          </ul>
        </template>

        <!-- run details -->
        <div class="ui divider"/>
        <div class="left-list-header">{{ locale.runDetails }}</div>
        <ul class="ui relaxed list">
          <li class="item">
            <a class="flex-text-block" :href="`${run.link}/workflow`">
              <SvgIcon name="octicon-file-code" class="tw-text-text"/>
              <span class="gt-ellipsis">{{ locale.workflowFile }}</span>
            </a>
          </li>
        </ul>
      </div>

      <div class="action-view-right">
        <ActionRunSummaryView
          v-if="!props.jobId"
          :store="store"
          :locale="locale"
        />
        <ActionRunJobView
          v-else
          :store="store"
          :locale="locale"
          :view-url="props.viewUrl"
          :job-id="props.jobId"
        />
      </div>
    </div>
  </div>
</template>
<style scoped>
.action-view-body {
  padding-top: 12px;
  padding-bottom: 12px;
  display: flex;
  gap: 12px;
}

/* ================ */
/* action view header */

.action-view-header {
  margin-top: 8px;
}

.action-info-summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.action-info-summary-title {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.action-info-summary-title-text {
  font-size: 20px;
  margin: 0;
  flex: 1;
  overflow-wrap: anywhere;
}

.action-info-summary .ui.button {
  margin: 0;
  white-space: nowrap;
}

.action-info-summary > .flex-text-block {
  gap: 8px;
}

.action-commit-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 5px;
  margin-left: 28px;
}

.attempt-switcher.ui.dropdown > .menu.attempt-switcher-menu {
  position: absolute;
  right: 0;
  top: calc(100% + 10px);
  margin-top: 0;
  min-width: 300px;
  padding: 0;
  border: 1px solid var(--color-secondary);
  border-radius: var(--border-radius);
  background: var(--color-box-body);
  box-shadow: 0 8px 24px var(--color-shadow);
  z-index: 10;
}

.attempt-switcher-menu > .attempt-switcher-item {
  padding: 12px 14px;
  margin: 0;
}

.attempt-switcher-menu > .attempt-switcher-item:not(:last-child) {
  border-bottom: 1px solid var(--color-secondary);
}

.attempt-switcher-menu > .attempt-switcher-item:hover {
  background: var(--color-hover);
}

.attempt-switcher-menu > .attempt-switcher-item.selected {
  background: var(--color-active);
  font-weight: var(--font-weight-semibold);
}

.attempt-switcher-item-meta {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 6px;
  color: var(--color-text-light-2);
  font-size: 13px;
  line-height: 1.4;
  white-space: nowrap;
}

@media (max-width: 767.98px) {
  .action-commit-summary {
    margin-left: 0;
    margin-top: 8px;
  }
}

/* ================ */
/* action view left */

.action-view-left {
  width: 30%;
  max-width: 400px;
  position: sticky;
  top: 12px;

  /* about 12px top padding + 12px bottom padding + 37px footer height,
  TODO: need to use JS to calculate the height for better scrolling experience*/
  max-height: calc(100vh - 62px);

  overflow-y: auto;
  background: var(--color-body);
  z-index: 2; /* above .job-info-header */
}

@media (max-width: 767.98px) {
  .action-view-left {
    position: static; /* can not sticky because multiple jobs would overlap into right view */
    max-height: unset;
  }
}

.left-list-header {
  font-size: 13px;
  color: var(--color-text-light-2);
}

.action-view-left .ui.relaxed.list {
  margin: var(--gap-block) 0;
  padding-left: 10px;
}

.job-brief-item {
  padding: 6px 10px;
  border-radius: var(--border-radius);
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: var(--gap-block);
}

.job-brief-item:hover {
  background-color: var(--color-hover);
}

.job-brief-item.selected {
  font-weight: var(--font-weight-bold);
  background-color: var(--color-active);
}

/* ================ */
/* action view right */

.action-view-right {
  flex: 1;
  color: var(--color-console-fg-subtle);
  max-height: 100%;
  width: 70%;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--color-console-border);
  border-radius: var(--border-radius);
  background: var(--color-console-bg);
}

/* begin fomantic button overrides */

.action-view-right .ui.button,
.action-view-right .ui.button:focus {
  background: transparent;
  color: var(--color-console-fg-subtle);
}

.action-view-right .ui.button:hover {
  background: var(--color-console-hover-bg);
  color: var(--color-console-fg);
}

.action-view-right .ui.button:active {
  background: var(--color-console-active-bg);
  color: var(--color-console-fg);
}

/* end fomantic button overrides */

@media (max-width: 767.98px) {
  .action-view-body {
    flex-direction: column;
  }
  .action-view-left, .action-view-right {
    width: 100%;
  }
  .action-view-left {
    max-width: none;
  }
}
</style>
