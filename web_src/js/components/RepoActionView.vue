<script setup lang="ts">
import {SvgIcon} from '../svg.ts';
import ActionStatusIcon from './ActionStatusIcon.vue';
import {computed, ref, toRefs} from 'vue';
import {POST, DELETE} from '../modules/fetch.ts';
import ActionRunSummaryView from './ActionRunSummaryView.vue';
import ActionRunJobView from './ActionRunJobView.vue';
import type {ActionsJob, ActionsRunAttempt} from '../modules/gitea-actions.ts';
import {buildJobsByParentJobID, createActionRunViewStore} from './ActionRunView.ts';
import {buildArtifactTooltipHtml} from './ActionRunArtifacts.ts';

defineOptions({
  name: 'RepoActionView',
});

const props = defineProps<{
  jobId: number;
  actionsViewUrl: string;
  locale: Record<string, any>;
}>();

const locale = props.locale;
const store = createActionRunViewStore(props.actionsViewUrl);
const {currentRun: run, runArtifacts: artifacts} = toRefs(store.viewData);

type JobListItem = {
  job: ActionsJob;
  depth: number;
  hasChildren: boolean;
};

// Caller jobs default to collapsed. Membership in this set means "user has manually expanded this caller"
const expandedJobIDs = ref(new Set<number>());

function toggleExpandedJob(jobID: number) {
  const next = new Set(expandedJobIDs.value);
  if (next.has(jobID)) {
    next.delete(jobID);
  } else {
    next.add(jobID);
  }
  expandedJobIDs.value = next;
}

// When a child job is currently selected, force-expand the chain of caller ancestors
const forcedExpandedJobIDs = computed(() => {
  const expanded = new Set<number>();
  if (!props.jobId) return expanded;
  const jobsByID = new Map((run.value.jobs || []).map((job) => [job.id, job]));
  let cur = jobsByID.get(props.jobId);
  while (cur?.parentJobID) {
    expanded.add(cur.parentJobID);
    cur = jobsByID.get(cur.parentJobID);
  }
  return expanded;
});

function isJobCollapsed(jobID: number) {
  return !expandedJobIDs.value.has(jobID) && !forcedExpandedJobIDs.value.has(jobID);
}

const visibleJobListItems = computed<JobListItem[]>(() => {
  const jobs = [...(run.value.jobs || [])].sort((a, b) => a.id - b.id);
  const childrenByParent = buildJobsByParentJobID(jobs);

  const result: JobListItem[] = [];
  const stack: Array<{job: ActionsJob; depth: number}> = [];
  const top = childrenByParent.get(0) || [];
  for (let i = top.length - 1; i >= 0; i--) stack.push({job: top[i], depth: 0});

  while (stack.length > 0) {
    const {job, depth} = stack.pop()!;
    const children = childrenByParent.get(job.id) || [];
    const hasChildren = children.length > 0;
    result.push({job, depth, hasChildren});
    if (hasChildren && isJobCollapsed(job.id)) continue;
    for (let i = children.length - 1; i >= 0; i--) stack.push({job: children[i], depth: depth + 1});
  }
  return result;
});

function formatAttemptTitle(attempt: ActionsRunAttempt) {
  return attempt.latest ? `${locale.latestAttempt} #${attempt.attempt}` : `${locale.attempt} #${attempt.attempt}`;
}

function formatCurrentAttemptTitle(attempt: ActionsRunAttempt) {
  return attempt.latest ? `${locale.latest} #${attempt.attempt}` : formatAttemptTitle(attempt);
}

function buildArtifactLink(name: string) {
  const searchString = run.value.runAttempt > 0 ? `?attempt=${run.value.runAttempt}` : '';
  return `${run.value.link}/artifacts/${encodeURIComponent(name)}${searchString}`;
}

function cancelRun() {
  POST(`${run.value.link}/cancel`);
}

function approveRun() {
  POST(`${run.value.link}/approve`);
}

async function deleteArtifact(name: string) {
  if (!window.confirm(locale.confirmDeleteArtifact.replace('%s', name))) return;
  await DELETE(buildArtifactLink(name));
  await store.forceReloadCurrentRun();
}
</script>
<template>
  <!-- make the view container full width to make users easier to read logs -->
  <div class="ui fluid container">
    <div class="action-view-header">
      <div class="action-info-summary">
        <div class="action-info-summary-title">
          <ActionStatusIcon :locale-status="locale.status[run.status]" :status="run.status" :size="20" icon-variant="circle-fill"/>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <h2 class="action-info-summary-title-text" v-html="run.titleHTML"/>
        </div>
        <div class="flex-text-block tw-shrink-0 tw-flex-wrap">
          <button class="ui basic small compact button primary" @click="approveRun()" v-if="run.canApprove">
            {{ locale.approve }}
          </button>
          <button class="ui small compact button tw-text-red" @click="cancelRun()" v-else-if="run.canCancel">
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
          <div v-if="run.attempts.length > 1" class="ui dropdown basic small compact button">
            <div class="flex-text-inline">
              <SvgIcon name="octicon-history" :size="14"/>
              <span>{{ formatCurrentAttemptTitle(run.attempts.find((attempt) => attempt.current)!) }}</span>
            </div>
            <SvgIcon name="octicon-triangle-down" :size="14" class="dropdown icon"/>
            <div class="menu">
              <a
                v-for="attempt in run.attempts"
                :key="attempt.attempt"
                class="item tw-flex tw-flex-col tw-gap-2"
                :class="attempt.current ? 'selected' : ''"
                :href="attempt.link"
              >
                <div class="flex-text-block">
                  <SvgIcon name="octicon-check" :size="14" :class="{'tw-invisible': !Boolean(attempt.current)}"/>
                  <strong class="tw-text-sm gt-ellipsis">{{ formatAttemptTitle(attempt) }}</strong>
                </div>
                <div class="flex-text-block tw-pl-[20px]">
                  <span class="flex-text-inline tw-flex-shrink-0">
                    <ActionStatusIcon :locale-status="locale.status[attempt.status]" :status="attempt.status" :size="14" class="flex-text-block" icon-variant="circle-fill"/>
                    <span>{{ locale.status[attempt.status] }}</span>
                  </span>
                  <span>•</span>
                  <relative-time :datetime="attempt.triggeredAt" prefix=""/>
                  <span>•</span>
                  <span class="gt-ellipsis">{{ attempt.triggerUserName }}</span>
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
        <div class="flex-items-block action-view-sidebar-list">
          <a class="item silenced" :href="run.viewLink" :class="!props.jobId ? 'selected' : ''">
            <SvgIcon name="octicon-home"/>
            <span class="gt-ellipsis">{{ locale.summary }}</span>
          </a>
        </div>

        <!-- jobs list -->
        <div class="ui divider"/>
        <div class="left-list-header">{{ locale.allJobs }}</div>
        <div class="flex-items-block action-view-sidebar-list">
          <div
            class="item job-brief-item"
            :class="{'selected': props.jobId === item.job.id}"
            :style="{paddingLeft: `${10 + item.depth * 16}px`}"
            v-for="item in visibleJobListItems"
            :key="item.job.id"
          >
            <a class="tw-contents silenced" :href="item.job.link">
              <ActionStatusIcon :locale-status="locale.status[item.job.status]" :status="item.job.status" icon-variant="circle-fill"/>
              <span class="tw-min-w-0 gt-ellipsis">{{ item.job.name }}</span>
              <SvgIcon name="octicon-sync" role="button" :data-tooltip-content="locale.rerun" class="job-rerun-button tw-cursor-pointer link-action interact-fg" :data-url="`${run.link}/jobs/${item.job.id}/rerun`" v-if="item.job.canRerun"/>
              <span class="job-duration">{{ item.job.duration }}</span>
            </a>
            <button
              v-if="item.hasChildren"
              type="button"
              class="job-brief-toggle"
              :class="{'collapsed': isJobCollapsed(item.job.id)}"
              @click="toggleExpandedJob(item.job.id)"
              :title="isJobCollapsed(item.job.id) ? locale.expandCallerJobs : locale.collapseCallerJobs"
              :aria-label="isJobCollapsed(item.job.id) ? locale.expandCallerJobs : locale.collapseCallerJobs"
              :aria-expanded="!isJobCollapsed(item.job.id)"
            >
              <SvgIcon name="octicon-chevron-down" :size="14"/>
            </button>
          </div>
        </div>

        <!-- artifacts list -->
        <template v-if="artifacts.length > 0">
          <div class="ui divider"/>
          <div class="left-list-header">{{ locale.artifactsTitle }} ({{ artifacts.length }})</div>
          <div class="flex-items-block action-view-sidebar-list">
            <div class="item" v-for="artifact in artifacts" :key="artifact.name">
              <template v-if="artifact.status !== 'expired'">
                <a
                  class="tw-flex-1 tw-min-w-0 flex-text-block silenced" target="_blank"
                  :href="buildArtifactLink(artifact.name)"
                  :data-tooltip-content="buildArtifactTooltipHtml(artifact, locale.artifactExpiresAt)"
                  data-tooltip-render="html"
                  data-tooltip-placement="top-end"
                >
                  <SvgIcon name="octicon-file" class="tw-text-text-light"/>
                  <span class="tw-flex-1 gt-ellipsis">{{ artifact.name }}</span>
                </a>
                <a v-if="run.canDeleteArtifact" class="silenced" @click="deleteArtifact(artifact.name)">
                  <SvgIcon name="octicon-trash"/>
                </a>
              </template>
              <span v-else class="flex-text-block tw-flex-1 tw-text-text-light-2">
                <SvgIcon name="octicon-file-removed"/>
                <span class="tw-flex-1 gt-ellipsis">{{ artifact.name }}</span>
                <span class="ui label tw-flex-shrink-0">{{ locale.artifactExpired }}</span>
              </span>
            </div>
          </div>
        </template>

        <!-- run details -->
        <div class="ui divider"/>
        <div class="left-list-header">{{ locale.runDetails }}</div>
        <div class="flex-items-block action-view-sidebar-list">
          <div class="item">
            <a class="flex-text-block silenced" :href="`${run.link}/workflow`">
              <SvgIcon name="octicon-file-code" class="tw-text-text"/>
              <span class="gt-ellipsis">{{ locale.workflowFile }}</span>
            </a>
          </div>
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
          :actions-view-url="props.actionsViewUrl"
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
  font-weight: var(--font-weight-semibold);
  color: var(--color-text-light-2);
}

.action-view-sidebar-list {
  margin: var(--gap-block) 0;
}

.action-view-sidebar-list:first-child {
  margin-top: 0;
}

.action-view-sidebar-list > .item {
  padding: 6px 10px;
  border-radius: var(--border-radius);
}

.action-view-sidebar-list > .item:hover {
  background-color: var(--color-hover);
}

.action-view-sidebar-list > .item.selected {
  font-weight: var(--font-weight-bold);
  background-color: var(--color-active);
}

.job-brief-toggle {
  border: none;
  padding: 0;
  background: transparent;
  cursor: pointer;
  color: inherit;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  /* the icon is always chevron-down; flip to chevron-up when expanded */
  transition: transform 0.15s ease;
  /* sit right after the job name; rerun/duration float to the right via auto-margin */
  order: 1;
}

.job-brief-toggle:not(.collapsed) {
  transform: rotate(180deg);
}

/* push rerun/duration to the right edge; only one is visible at a time (hover swap),
   the visible one absorbs the free space via auto-margin */
.action-view-sidebar-list > .item .job-rerun-button,
.action-view-sidebar-list > .item .job-duration {
  order: 2;
  margin-left: auto;
}

/* the re-run button replaces the duration on hover or job-link focus */
.action-view-sidebar-list > .item .job-rerun-button {
  display: none;
}

.action-view-sidebar-list > .item:hover .job-rerun-button,
.action-view-sidebar-list > .item:has(a:focus) .job-rerun-button {
  display: inline-flex;
}

/* only swap out the duration when a re-run button exists to take its place */
.action-view-sidebar-list > .item:hover .job-rerun-button ~ .job-duration,
.action-view-sidebar-list > .item:has(a:focus) .job-rerun-button ~ .job-duration {
  display: none;
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
