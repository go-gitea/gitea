<script setup lang="ts">
import {SvgIcon} from '../svg.ts';
import ActionStatusIcon from './ActionStatusIcon.vue';
import {computed, onBeforeUnmount, ref, toRefs, watch} from 'vue';
import {resetActionFavicon, syncActionRunFavicon} from '../modules/favicon-status.ts';
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
const visibleJobSummaries = computed(() => {
  const summaries = run.value.jobSummaries || [];
  if (!props.jobId) return summaries;
  return summaries.filter((summary) => summary.jobId === props.jobId);
});

type JobListItem = {
  job: ActionsJob;
  depth: number;
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
    result.push({job, depth});
    if (children.length > 0 && isJobCollapsed(job.id)) continue;
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

const backLink = computed(() => {
  if (run.value.pullRequest) {
    return {href: run.value.pullRequest.link, prefix: locale.backToPullRequest, name: run.value.pullRequest.index};
  }
  if (run.value.workflowLink) {
    return {href: run.value.workflowLink, prefix: locale.backToWorkflow, name: run.value.workflowID.replace(/\.(yml|yaml)$/i, '')};
  }
  return null;
});

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

watch(() => run.value.status, (status) => {
  syncActionRunFavicon(status);
});

onBeforeUnmount(() => {
  resetActionFavicon();
});
</script>
<template>
  <!-- make the view container full width to make users easier to read logs -->
  <div class="ui fluid container">
    <div class="action-view-header">
      <a v-if="backLink" class="action-view-back silenced" :href="backLink.href">
        <SvgIcon name="octicon-arrow-left" :size="14"/>
        <span>{{ backLink.prefix }} <span class="action-view-back-name">{{ backLink.name }}</span></span>
      </a>
      <div class="action-info-summary">
        <div class="action-info-summary-title">
          <ActionStatusIcon :locale-status="locale.status[run.status]" :status="run.status" :size="22" icon-variant="circle-fill"/>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <h2 class="action-info-summary-title-text" v-html="run.titleHTML"/>
          <span class="action-info-summary-title-index">#{{ run.index }}</span>
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
            <!-- Callers have no log page of their own; the whole row toggles expansion
                 (matches GitHub Actions, where caller rows are not navigation targets). -->
            <button
              v-if="item.job.isReusableCaller"
              type="button"
              class="tw-contents caller-row-toggle"
              @click="toggleExpandedJob(item.job.id)"
              :title="isJobCollapsed(item.job.id) ? locale.expandCallerJobs : locale.collapseCallerJobs"
              :aria-label="isJobCollapsed(item.job.id) ? locale.expandCallerJobs : locale.collapseCallerJobs"
              :aria-expanded="!isJobCollapsed(item.job.id)"
            >
              <ActionStatusIcon :locale-status="locale.status[item.job.status]" :status="item.job.status" icon-variant="circle-fill"/>
              <span class="tw-min-w-0 gt-ellipsis">{{ item.job.name }}</span>
              <span class="job-duration">{{ item.job.duration }}</span>
              <SvgIcon name="octicon-chevron-down" :size="14" class="job-brief-toggle-icon" :class="{'collapsed': isJobCollapsed(item.job.id)}"/>
            </button>
            <a v-else class="tw-contents silenced" :href="item.job.link">
              <ActionStatusIcon :locale-status="locale.status[item.job.status]" :status="item.job.status" icon-variant="circle-fill"/>
              <span class="tw-min-w-0 gt-ellipsis">{{ item.job.name }}</span>
              <SvgIcon name="octicon-sync" role="button" :data-tooltip-content="locale.rerun" class="job-rerun-button tw-cursor-pointer link-action interact-fg" :data-url="`${run.link}/jobs/${item.job.id}/rerun`" v-if="item.job.canRerun"/>
              <span class="job-duration">{{ item.job.duration }}</span>
            </a>
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
              <span v-else class="flex-text-block tw-flex-1 tw-min-w-0 tw-text-text-light-2">
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
        <div class="action-view-right-panel">
          <ActionRunSummaryView
            v-if="!props.jobId"
            :store="store"
            :locale="locale"
            :artifact-count="artifacts.length"
          />
          <ActionRunJobView
            v-else
            :store="store"
            :locale="locale"
            :actions-view-url="props.actionsViewUrl"
            :job-id="props.jobId"
          />
        </div>
        <div v-if="visibleJobSummaries.length" class="action-view-right-panel job-summary-section">
          <div class="job-summary-section-header">
            {{ locale.jobSummaries }}
          </div>
          <div class="job-summary-list">
            <div v-for="s in visibleJobSummaries" :key="s.jobId" class="job-summary-item">
              <div class="job-summary-header">
                <strong class="gt-ellipsis">{{ s.jobName || `Job ${s.jobId}` }}</strong>
              </div>
              <!-- eslint-disable-next-line vue/no-v-html -->
              <div class="markup job-summary-body" v-html="s.summaryHTML"/>
            </div>
          </div>
        </div>
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
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}

.action-view-back {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  gap: 4px;
  font-size: 13px;
  color: var(--color-text-light-1);
}

.action-view-back:hover {
  color: var(--color-primary);
}

.action-view-back-name {
  font-weight: var(--font-weight-bold);
  color: var(--color-text);
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
  overflow-wrap: anywhere;
}

.action-info-summary-title-index {
  font-size: 20px;
  color: var(--color-text-light-2);
  flex: 1;
}

.action-info-summary .ui.button {
  margin: 0;
  white-space: nowrap;
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

.caller-row-toggle {
  border: none;
  padding: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: inherit;
}

.job-brief-toggle-icon {
  flex-shrink: 0;
  transition: transform 0.15s ease;
  /* sit between name and duration; duration uses order:2 with margin-left:auto to float right */
  order: 1;
}

.job-brief-toggle-icon:not(.collapsed) {
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
  gap: 12px;
}

.action-view-right-panel {
  flex: 1; /* fill the right column so the summary graph stretches even without a job-summary section */
  border: 1px solid var(--color-console-border);
  border-radius: var(--border-radius);
  background: var(--color-console-bg);
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* begin fomantic button overrides */

.action-view-right-panel .ui.button,
.action-view-right-panel .ui.button:focus {
  background: transparent;
  color: var(--color-console-fg-subtle);
}

.action-view-right-panel .ui.button:hover {
  background: var(--color-console-hover-bg);
  color: var(--color-console-fg);
}

.action-view-right-panel .ui.button:active {
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

.job-summary-section {
  flex: 0 0 auto; /* size to its content; let the summary panel keep the remaining height */
  overflow: hidden;
}

.job-summary-section-header {
  padding: 12px;
  border-bottom: 1px solid var(--color-console-border);
  background: var(--color-console-bg);
  color: var(--color-console-fg);
  font-weight: var(--font-weight-semibold);
}

.job-summary-list {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.job-summary-item {
  padding: 12px;
  border-radius: var(--border-radius);
  background: var(--color-console-hover-bg);
  border: 1px solid var(--color-console-border);
}

.job-summary-header {
  color: var(--color-console-fg);
  margin-bottom: 8px;
}

.job-summary-body {
  color: var(--color-console-fg);
}
</style>
