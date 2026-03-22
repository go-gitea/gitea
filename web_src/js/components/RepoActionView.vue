<script setup lang="ts">
import {SvgIcon} from '../svg.ts';
import ActionRunStatus from './ActionRunStatus.vue';
import {toRefs} from 'vue';
import {POST, DELETE} from '../modules/fetch.ts';
import ActionRunSummaryView from './ActionRunSummaryView.vue';
import ActionRunJobView from './ActionRunJobView.vue';
import {createActionRunViewStore} from "./ActionRunView.ts";

defineOptions({
  name: 'RepoActionView',
});

const props = defineProps<{
  runId: number;
  jobId: number;
  actionsUrl: string;
  locale: Record<string, any>;
}>();

const locale = props.locale;
const store = createActionRunViewStore(props.actionsUrl, props.runId);
const {currentRun: run , runArtifacts: artifacts} = toRefs(store.viewData);

function cancelRun() {
  POST(`${run.value.link}/cancel`);
}

function approveRun() {
  POST(`${run.value.link}/approve`);
}

async function deleteArtifact(name: string) {
  if (!window.confirm(locale.confirmDeleteArtifact.replace('%s', name))) return;
  await DELETE(`${run.value.link}/artifacts/${encodeURIComponent(name)}`);
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
          <template v-else-if="run.canRerun">
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
        </div>
      </div>
      <div class="action-commit-summary">
        <span><a class="muted" :href="run.workflowLink"><b>{{ run.workflowID }}</b></a>:</span>
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
        <div class="job-group-section">
          <a class="job-brief-item" :href="run.link" :class="!props.jobId ? 'selected' : ''">
            <div class="job-brief-item-left">
              <SvgIcon name="octicon-list-unordered" class="tw-mr-2"/>
              <span class="job-brief-name tw-mx-2 gt-ellipsis">{{ locale.summary }}</span>
            </div>
          </a>
          <div class="ui divider"/>
          <div class="job-brief-list">
            <div class="left-list-header">{{ locale.allJobs }}</div>
            <a class="job-brief-item" :href="run.link+'/jobs/'+job.id" :class="props.jobId === job.id ? 'selected' : ''" v-for="job in run.jobs" :key="job.id">
              <div class="job-brief-item-left">
                <ActionRunStatus :locale-status="locale.status[job.status]" :status="job.status"/>
                <span class="job-brief-name tw-mx-2 gt-ellipsis">{{ job.name }}</span>
              </div>
              <span class="job-brief-item-right">
                <SvgIcon name="octicon-sync" role="button" :data-tooltip-content="locale.rerun" class="job-brief-rerun tw-mx-2 link-action interact-fg" :data-url="`${run.link}/jobs/${job.id}/rerun`" v-if="job.canRerun"/>
                <span class="step-summary-duration">{{ job.duration }}</span>
              </span>
            </a>
          </div>
        </div>
        <div class="job-artifacts" v-if="artifacts.length > 0">
          <div class="ui divider"/>
          <div class="left-list-header">{{ locale.artifactsTitle }} ({{ artifacts.length }})</div>
          <ul class="job-artifacts-list">
            <template v-for="artifact in artifacts" :key="artifact.name">
              <li class="job-artifacts-item">
                <template v-if="artifact.status !== 'expired'">
                  <a class="flex-text-inline" target="_blank" :href="run.link+'/artifacts/'+artifact.name">
                    <SvgIcon name="octicon-file" class="tw-text-text"/>
                    <span class="gt-ellipsis">{{ artifact.name }}</span>
                  </a>
                  <a v-if="run.canDeleteArtifact" @click="deleteArtifact(artifact.name)">
                    <SvgIcon name="octicon-trash" class="tw-text-text"/>
                  </a>
                </template>
                <span v-else class="flex-text-inline tw-text-grey-light">
                  <SvgIcon name="octicon-file"/>
                  <span class="gt-ellipsis">{{ artifact.name }}</span>
                  <span class="ui label tw-text-grey-light tw-flex-shrink-0">{{ locale.artifactExpired }}</span>
                </span>
              </li>
            </template>
          </ul>
        </div>
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
          :run-id="props.runId"
          :job-id="props.jobId"
          :actions-url="props.actionsUrl"
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

.action-commit-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 5px;
  margin-left: 28px;
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
  max-height: 100vh;
  overflow-y: auto;
  background: var(--color-body);
  z-index: 2; /* above .job-info-header */
}

@media (max-width: 767.98px) {
  .action-view-left {
    position: static; /* can not sticky because multiple jobs would overlap into right view */
  }
}

.left-list-header {
  font-size: 12px;
  color: var(--color-grey);
}

.job-artifacts-item {
  margin: 5px 0;
  padding: 6px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.job-artifacts-list {
  padding-left: 4px;
  list-style: none;
}

.job-brief-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.job-brief-item {
  padding: 6px 10px;
  border-radius: var(--border-radius);
  text-decoration: none;
  display: flex;
  flex-wrap: nowrap;
  justify-content: space-between;
  align-items: center;
  color: var(--color-text);
}

.job-brief-item:hover {
  background-color: var(--color-hover);
}

.job-brief-item.selected {
  font-weight: var(--font-weight-bold);
  background-color: var(--color-active);
}

.job-brief-item:first-of-type {
  margin-top: 0;
}

.job-brief-item .job-brief-rerun {
  cursor: pointer;
}

.job-brief-item .job-brief-item-left {
  display: flex;
  width: 100%;
  min-width: 0;
}

.job-brief-item .job-brief-item-left span {
  display: flex;
  align-items: center;
}

.job-brief-item .job-brief-item-left .job-brief-name {
  display: block;
  width: 70%;
}

.job-brief-item .job-brief-item-right {
  display: flex;
  align-items: center;
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
  align-self: flex-start;
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
