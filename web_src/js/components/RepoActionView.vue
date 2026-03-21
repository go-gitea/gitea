<script lang="ts">
import {SvgIcon} from '../svg.ts';
import ActionRunStatus from './ActionRunStatus.vue';
import {defineComponent, type PropType} from 'vue';
import {POST, DELETE} from '../modules/fetch.ts';
import type {IntervalId} from '../types.ts';
import type {ActionsRunStatus, ActionsJob} from '../modules/gitea-actions.ts';
import ActionRunSummaryView from './ActionRunSummaryView.vue';
import ActionRunJobView from './ActionRunJobView.vue';

export default defineComponent({
  name: 'RepoActionView',
  components: {
    SvgIcon,
    ActionRunStatus,
    ActionRunSummaryView,
    ActionRunJobView,
  },
  props: {
    runId: {type: Number, required: true},
    jobId: {type: Number, required: true},
    actionsURL: {type: String, required: true},
    locale: {
      type: Object as PropType<Record<string, any>>,
      default: null,
    },
  },

  data() {
    return {
      // internal state
      loadingAbortController: null as AbortController | null,
      intervalID: null as IntervalId | null,
      artifacts: [] as Array<Record<string, any>>,
      // provided by backend
      run: {
        link: '',
        title: '',
        titleHTML: '',
        status: '' as ActionsRunStatus, // do not show the status before initialized, otherwise it would show an incorrect "error" icon
        canCancel: false,
        canApprove: false,
        canRerun: false,
        canDeleteArtifact: false,
        done: false,
        workflowID: '',
        workflowLink: '',
        isSchedule: false,
        duration: '',
        triggeredAt: 0,
        triggerEvent: '',
        jobs: [
          // {
          //   id: 0,
          //   name: '',
          //   status: '',
          //   canRerun: false,
          //   duration: '',
          // },
        ] as Array<ActionsJob>,
        commit: {
          localeCommit: '',
          localePushedBy: '',
          shortSHA: '',
          link: '',
          pusher: {
            displayName: '',
            link: '',
          },
          branch: {
            name: '',
            link: '',
            isDeleted: false,
          },
        },
      },
    };
  },

  computed: {
    isSummaryMode(): boolean {
      return this.jobId === 0;
    },
    runTriggeredAtISO(): string {
      const t = this.run.triggeredAt;
      return t ? new Date(t * 1000).toISOString() : '';
    },
    runTriggerEventLabel(): string {
      const e = this.run.triggerEvent;
      if (!e) return '';
      return e.replace(/_/g, ' ');
    },
  },

  async mounted() {
    // load run data and then auto-reload periodically
    await this.loadRun();
    this.intervalID = setInterval(() => this.loadRun(), 1000);
  },

  unmounted() {
    // clear the interval timer when the component is unmounted
    // even our page is rendered once, not spa style
    if (this.intervalID) {
      clearInterval(this.intervalID);
      this.intervalID = null;
    }
  },

  methods: {
    // cancel a run
    cancelRun() {
      POST(`${this.run.link}/cancel`);
    },
    // approve a run
    approveRun() {
      POST(`${this.run.link}/approve`);
    },
    async deleteArtifact(name: string) {
      if (!window.confirm(this.locale.confirmDeleteArtifact.replace('%s', name))) return;
      // TODO: should escape the "name"?
      await DELETE(`${this.run.link}/artifacts/${name}`);
      await this.loadRunForce();
    },
    async fetchRunData(abortController: AbortController) {
      const url = `${this.actionsURL}/runs/${this.runId}`;
      const resp = await POST(url, {
        signal: abortController.signal,
        data: {logCursors: []},
      });
      return await resp.json();
    },
    async loadRunForce() {
      this.loadingAbortController?.abort();
      this.loadingAbortController = null;
      await this.loadRun();
    },
    async loadRun() {
      if (this.loadingAbortController) return;
      const abortController = new AbortController();
      this.loadingAbortController = abortController;
      try {
        const job = await this.fetchRunData(abortController);
        if (this.loadingAbortController !== abortController) return;

        this.artifacts = job.artifacts || [];
        this.run = job.state.run;
        // clear the interval timer if the job is done
        if (this.run.done && this.intervalID) {
          clearInterval(this.intervalID);
          this.intervalID = null;
        }
      } catch (e) {
        // avoid network error while unloading page, and ignore "abort" error
        if (e instanceof TypeError || abortController.signal.aborted) return;
        throw e;
      } finally {
        if (this.loadingAbortController === abortController) this.loadingAbortController = null;
      }
    },

  },
});
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
          <button class="ui basic small compact button link-action" :data-url="`${run.link}/rerun`" v-else-if="run.canRerun">
            {{ locale.rerun_all }}
          </button>
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
          <div class="job-brief-list">
            <a class="job-brief-item" :href="run.link" :class="isSummaryMode ? 'selected' : ''">
              <div class="job-brief-item-left">
                <SvgIcon name="octicon-list-unordered" class="tw-mr-2"/>
                <span class="job-brief-name tw-mx-2 gt-ellipsis">{{ locale.summary }}</span>
              </div>
            </a>
            <div class="ui divider tw-mt-2 tw-mb-1"/>
            <div class="tw-text-sm tw-text-grey tw-mt-1 tw-mb-1">{{ locale.allJobs }}</div>
            <a class="job-brief-item" :href="run.link+'/jobs/'+job.id" :class="jobId === job.id ? 'selected' : ''" v-for="job in run.jobs" :key="job.id">
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
          <div class="job-artifacts-title">
            {{ locale.artifactsTitle }}
          </div>
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
          v-if="isSummaryMode"
          :run="run"
          :artifacts="artifacts"
          :locale="locale"
          :run-triggered-at-iso="runTriggeredAtISO"
          :run-trigger-event-label="runTriggerEventLabel"
        />
        <ActionRunJobView
          v-else
          ref="jobView"
          :run-id="runId"
          :job-id="jobId"
          :actions-u-r-l="actionsURL"
          :locale="locale"
          :run="run"
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

.job-artifacts-title {
  font-size: 18px;
  margin-top: 16px;
  padding: 16px 10px 0 20px;
  border-top: 1px solid var(--color-secondary);
}

.job-artifacts-item {
  margin: 5px 0;
  padding: 6px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.job-artifacts-list {
  padding-left: 12px;
  list-style: none;
}

.job-brief-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.job-brief-item {
  padding: 10px;
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

<style> /* eslint-disable-line vue-scoped-css/enforce-style-type */
/* some elements are not managed by vue, so we need to use global style */
.job-step-section {
  margin: 10px;
}

.job-step-section .job-step-logs {
  font-family: var(--fonts-monospace);
  margin: 8px 0;
  font-size: 12px;
}

.job-step-section .job-step-logs .job-log-line {
  display: flex;
}

.job-log-line:hover,
.job-log-line:target {
  background-color: var(--color-console-hover-bg);
}

.job-log-line:target {
  scroll-margin-top: 95px;
}

/* class names 'log-time-seconds' and 'log-time-stamp' are used in the method toggleTimeDisplay */
.job-log-line .line-num, .log-time-seconds {
  width: 48px;
  color: var(--color-text-light-3);
  text-align: right;
  user-select: none;
}

.job-log-line:target > .line-num {
  color: var(--color-primary);
  text-decoration: underline;
}

.log-time-seconds {
  padding-right: 2px;
}

.job-log-line .log-time,
.log-time-stamp {
  color: var(--color-text-light-3);
  margin-left: 10px;
  white-space: nowrap;
}

.job-step-logs .job-log-line .log-msg {
  flex: 1;
  white-space: break-spaces;
  margin-left: 10px;
  overflow-wrap: anywhere;
}

.job-step-logs .job-log-line .log-cmd-command {
  color: var(--color-ansi-blue);
}

.job-step-logs .job-log-line .log-cmd-error {
  color: var(--color-ansi-red);
}

/* selectors here are intentionally exact to only match fullscreen */

.full.height > .action-view-right {
  width: 100%;
  height: 100%;
  padding: 0;
  border-radius: 0;
}

.full.height > .action-view-right > .job-info-header {
  border-radius: 0;
}

.full.height > .action-view-right > .job-step-container {
  height: calc(100% - 60px);
  border-radius: 0;
}

.job-log-group .job-log-list .job-log-line .log-msg {
  margin-left: 2em;
}

.job-log-group-summary {
  position: relative;
}

.job-log-group-summary > .job-log-line {
  position: absolute;
  inset: 0;
  z-index: -1; /* to avoid hiding the triangle of the "details" element */
  overflow: hidden;
}
</style>
