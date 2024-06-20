<script>
import {SvgIcon} from '../svg.js';
import ActionRunStatus from './ActionRunStatus.vue';
import {createApp} from 'vue';
import {toggleElem} from '../utils/dom.js';
import {formatDatetime} from '../utils/time.js';
import {renderAnsi} from '../render/ansi.js';
import {GET, POST, DELETE} from '../modules/fetch.js';

const sfc = {
  name: 'RepoActionView',
  components: {
    SvgIcon,
    ActionRunStatus,
  },
  props: {
    runIndex: String,
    jobIndex: String,
    actionsURL: String,
    locale: Object,
  },

  data() {
    return {
      // internal state
      loading: false,
      intervalID: null,
      currentJobStepsStates: [],
      artifacts: [],
      onHoverRerunIndex: -1,
      menuVisible: false,
      isFullScreen: false,
      timeVisible: {
        'log-time-stamp': false,
        'log-time-seconds': false,
      },

      // provided by backend
      run: {
        link: '',
        title: '',
        status: '',
        canCancel: false,
        canApprove: false,
        canRerun: false,
        done: false,
        workflowID: '',
        workflowLink: '',
        isSchedule: false,
        jobs: [
          // {
          //   id: 0,
          //   name: '',
          //   status: '',
          //   canRerun: false,
          //   duration: '',
          // },
        ],
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
          },
        },
      },
      currentJob: {
        title: '',
        detail: '',
        steps: [
          // {
          //   summary: '',
          //   duration: '',
          //   status: '',
          // }
        ],
      },
    };
  },

  async mounted() {
    // load job data and then auto-reload periodically
    // need to await first loadJob so this.currentJobStepsStates is initialized and can be used in hashChangeListener
    await this.loadJob();
    this.intervalID = setInterval(this.loadJob, 1000);
    document.body.addEventListener('click', this.closeDropdown);
    this.hashChangeListener();
    window.addEventListener('hashchange', this.hashChangeListener);
  },

  beforeUnmount() {
    document.body.removeEventListener('click', this.closeDropdown);
    window.removeEventListener('hashchange', this.hashChangeListener);
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
    // get the active container element, either the `job-step-logs` or the `job-log-list` in the `job-log-group`
    getLogsContainer(idx) {
      const el = this.$refs.logs[idx];
      return el._stepLogsActiveContainer ?? el;
    },
    // begin a log group
    beginLogGroup(idx) {
      const el = this.$refs.logs[idx];

      const elJobLogGroup = document.createElement('div');
      elJobLogGroup.classList.add('job-log-group');

      const elJobLogGroupSummary = document.createElement('div');
      elJobLogGroupSummary.classList.add('job-log-group-summary');

      const elJobLogList = document.createElement('div');
      elJobLogList.classList.add('job-log-list');

      elJobLogGroup.append(elJobLogGroupSummary);
      elJobLogGroup.append(elJobLogList);
      el._stepLogsActiveContainer = elJobLogList;
    },
    // end a log group
    endLogGroup(idx) {
      const el = this.$refs.logs[idx];
      el._stepLogsActiveContainer = null;
    },

    // show/hide the step logs for a step
    toggleStepLogs(idx) {
      this.currentJobStepsStates[idx].expanded = !this.currentJobStepsStates[idx].expanded;
      if (this.currentJobStepsStates[idx].expanded) {
        this.loadJob(); // try to load the data immediately instead of waiting for next timer interval
      }
    },
    // cancel a run
    cancelRun() {
      POST(`${this.run.link}/cancel`);
    },
    // approve a run
    approveRun() {
      POST(`${this.run.link}/approve`);
    },

    createLogLine(line, startTime, stepIndex) {
      const div = document.createElement('div');
      div.classList.add('job-log-line');
      div.setAttribute('id', `jobstep-${stepIndex}-${line.index}`);
      div._jobLogTime = line.timestamp;

      const lineNumber = document.createElement('a');
      lineNumber.classList.add('line-num', 'muted');
      lineNumber.textContent = line.index;
      lineNumber.setAttribute('href', `#jobstep-${stepIndex}-${line.index}`);
      div.append(lineNumber);

      // for "Show timestamps"
      const logTimeStamp = document.createElement('span');
      logTimeStamp.className = 'log-time-stamp';
      const date = new Date(parseFloat(line.timestamp * 1000));
      const timeStamp = formatDatetime(date);
      logTimeStamp.textContent = timeStamp;
      toggleElem(logTimeStamp, this.timeVisible['log-time-stamp']);
      // for "Show seconds"
      const logTimeSeconds = document.createElement('span');
      logTimeSeconds.className = 'log-time-seconds';
      const seconds = Math.floor(parseFloat(line.timestamp) - parseFloat(startTime));
      logTimeSeconds.textContent = `${seconds}s`;
      toggleElem(logTimeSeconds, this.timeVisible['log-time-seconds']);

      const logMessage = document.createElement('span');
      logMessage.className = 'log-msg';
      logMessage.innerHTML = renderAnsi(line.message);
      div.append(logTimeStamp);
      div.append(logMessage);
      div.append(logTimeSeconds);

      return div;
    },

    appendLogs(stepIndex, logLines, startTime) {
      for (const line of logLines) {
        // TODO: group support: ##[group]GroupTitle , ##[endgroup]
        const el = this.getLogsContainer(stepIndex);
        el.append(this.createLogLine(line, startTime, stepIndex));
      }
    },

    async fetchArtifacts() {
      const resp = await GET(`${this.actionsURL}/runs/${this.runIndex}/artifacts`);
      return await resp.json();
    },

    async deleteArtifact(name) {
      if (!window.confirm(this.locale.confirmDeleteArtifact.replace('%s', name))) return;
      await DELETE(`${this.run.link}/artifacts/${name}`);
      await this.loadJob();
    },

    async fetchJob() {
      const logCursors = this.currentJobStepsStates.map((it, idx) => {
        // cursor is used to indicate the last position of the logs
        // it's only used by backend, frontend just reads it and passes it back, it and can be any type.
        // for example: make cursor=null means the first time to fetch logs, cursor=eof means no more logs, etc
        return {step: idx, cursor: it.cursor, expanded: it.expanded};
      });
      const resp = await POST(`${this.actionsURL}/runs/${this.runIndex}/jobs/${this.jobIndex}`, {
        data: {logCursors},
      });
      return await resp.json();
    },

    async loadJob() {
      if (this.loading) return;
      try {
        this.loading = true;

        let job, artifacts;
        try {
          [job, artifacts] = await Promise.all([
            this.fetchJob(),
            this.fetchArtifacts(), // refresh artifacts if upload-artifact step done
          ]);
        } catch (err) {
          if (err instanceof TypeError) return; // avoid network error while unloading page
          throw err;
        }

        this.artifacts = artifacts['artifacts'] || [];

        // save the state to Vue data, then the UI will be updated
        this.run = job.state.run;
        this.currentJob = job.state.currentJob;

        // sync the currentJobStepsStates to store the job step states
        for (let i = 0; i < this.currentJob.steps.length; i++) {
          if (!this.currentJobStepsStates[i]) {
            // initial states for job steps
            this.currentJobStepsStates[i] = {cursor: null, expanded: false};
          }
        }
        // append logs to the UI
        for (const logs of job.logs.stepsLog) {
          // save the cursor, it will be passed to backend next time
          this.currentJobStepsStates[logs.step].cursor = logs.cursor;
          this.appendLogs(logs.step, logs.lines, logs.started);
        }

        if (this.run.done && this.intervalID) {
          clearInterval(this.intervalID);
          this.intervalID = null;
        }
      } finally {
        this.loading = false;
      }
    },

    isDone(status) {
      return ['success', 'skipped', 'failure', 'cancelled'].includes(status);
    },

    isExpandable(status) {
      return ['success', 'running', 'failure', 'cancelled'].includes(status);
    },

    closeDropdown() {
      if (this.menuVisible) this.menuVisible = false;
    },

    toggleTimeDisplay(type) {
      this.timeVisible[`log-time-${type}`] = !this.timeVisible[`log-time-${type}`];
      for (const el of this.$refs.steps.querySelectorAll(`.log-time-${type}`)) {
        toggleElem(el, this.timeVisible[`log-time-${type}`]);
      }
    },

    toggleFullScreen() {
      this.isFullScreen = !this.isFullScreen;
      const fullScreenEl = document.querySelector('.action-view-right');
      const outerEl = document.querySelector('.full.height');
      const actionBodyEl = document.querySelector('.action-view-body');
      const headerEl = document.querySelector('#navbar');
      const contentEl = document.querySelector('.page-content.repository');
      const footerEl = document.querySelector('.page-footer');
      toggleElem(headerEl, !this.isFullScreen);
      toggleElem(contentEl, !this.isFullScreen);
      toggleElem(footerEl, !this.isFullScreen);
      // move .action-view-right to new parent
      if (this.isFullScreen) {
        outerEl.append(fullScreenEl);
      } else {
        actionBodyEl.append(fullScreenEl);
      }
    },
    async hashChangeListener() {
      const selectedLogStep = window.location.hash;
      if (!selectedLogStep) return;
      const [_, step, _line] = selectedLogStep.split('-');
      if (!this.currentJobStepsStates[step]) return;
      if (!this.currentJobStepsStates[step].expanded && this.currentJobStepsStates[step].cursor === null) {
        this.currentJobStepsStates[step].expanded = true;
        // need to await for load job if the step log is loaded for the first time
        // so logline can be selected by querySelector
        await this.loadJob();
      }
      const logLine = this.$refs.steps.querySelector(selectedLogStep);
      if (!logLine) return;
      logLine.querySelector('.line-num').click();
    },
  },
};

export default sfc;

export function initRepositoryActionView() {
  const el = document.querySelector('#repo-action-view');
  if (!el) return;

  // TODO: the parent element's full height doesn't work well now,
  // but we can not pollute the global style at the moment, only fix the height problem for pages with this component
  const parentFullHeight = document.querySelector('body > div.full.height');
  if (parentFullHeight) parentFullHeight.style.paddingBottom = '0';

  const view = createApp(sfc, {
    runIndex: el.getAttribute('data-run-index'),
    jobIndex: el.getAttribute('data-job-index'),
    actionsURL: el.getAttribute('data-actions-url'),
    locale: {
      approve: el.getAttribute('data-locale-approve'),
      cancel: el.getAttribute('data-locale-cancel'),
      rerun: el.getAttribute('data-locale-rerun'),
      rerun_all: el.getAttribute('data-locale-rerun-all'),
      scheduled: el.getAttribute('data-locale-runs-scheduled'),
      commit: el.getAttribute('data-locale-runs-commit'),
      pushedBy: el.getAttribute('data-locale-runs-pushed-by'),
      artifactsTitle: el.getAttribute('data-locale-artifacts-title'),
      areYouSure: el.getAttribute('data-locale-are-you-sure'),
      confirmDeleteArtifact: el.getAttribute('data-locale-confirm-delete-artifact'),
      showTimeStamps: el.getAttribute('data-locale-show-timestamps'),
      showLogSeconds: el.getAttribute('data-locale-show-log-seconds'),
      showFullScreen: el.getAttribute('data-locale-show-full-screen'),
      downloadLogs: el.getAttribute('data-locale-download-logs'),
      status: {
        unknown: el.getAttribute('data-locale-status-unknown'),
        waiting: el.getAttribute('data-locale-status-waiting'),
        running: el.getAttribute('data-locale-status-running'),
        success: el.getAttribute('data-locale-status-success'),
        failure: el.getAttribute('data-locale-status-failure'),
        cancelled: el.getAttribute('data-locale-status-cancelled'),
        skipped: el.getAttribute('data-locale-status-skipped'),
        blocked: el.getAttribute('data-locale-status-blocked'),
      },
    },
  });
  view.mount(el);
}
</script>
<template>
  <div class="ui container action-view-container">
    <div class="action-view-header">
      <div class="action-info-summary">
        <div class="action-info-summary-title">
          <ActionRunStatus :locale-status="locale.status[run.status]" :status="run.status" :size="20"/>
          <h2 class="action-info-summary-title-text">
            {{ run.title }}
          </h2>
        </div>
        <button class="ui basic small compact button primary" @click="approveRun()" v-if="run.canApprove">
          {{ locale.approve }}
        </button>
        <button class="ui basic small compact button red" @click="cancelRun()" v-else-if="run.canCancel">
          {{ locale.cancel }}
        </button>
        <button class="ui basic small compact button tw-mr-0 tw-whitespace-nowrap link-action" :data-url="`${run.link}/rerun`" v-else-if="run.canRerun">
          {{ locale.rerun_all }}
        </button>
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
          <a class="gt-ellipsis" :href="run.commit.branch.link">{{ run.commit.branch.name }}</a>
        </span>
      </div>
    </div>
    <div class="action-view-body">
      <div class="action-view-left">
        <div class="job-group-section">
          <div class="job-brief-list">
            <a class="job-brief-item" :href="run.link+'/jobs/'+index" :class="parseInt(jobIndex) === index ? 'selected' : ''" v-for="(job, index) in run.jobs" :key="job.id" @mouseenter="onHoverRerunIndex = job.id" @mouseleave="onHoverRerunIndex = -1">
              <div class="job-brief-item-left">
                <ActionRunStatus :locale-status="locale.status[job.status]" :status="job.status"/>
                <span class="job-brief-name tw-mx-2 gt-ellipsis">{{ job.name }}</span>
              </div>
              <span class="job-brief-item-right">
                <SvgIcon name="octicon-sync" role="button" :data-tooltip-content="locale.rerun" class="job-brief-rerun tw-mx-2 link-action" :data-url="`${run.link}/jobs/${index}/rerun`" v-if="job.canRerun && onHoverRerunIndex === job.id"/>
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
            <li class="job-artifacts-item" v-for="artifact in artifacts" :key="artifact.name">
              <a class="job-artifacts-link" target="_blank" :href="run.link+'/artifacts/'+artifact.name">
                <SvgIcon name="octicon-file" class="ui text black job-artifacts-icon"/>{{ artifact.name }}
              </a>
              <a v-if="run.canDeleteArtifact" @click="deleteArtifact(artifact.name)" class="job-artifacts-delete">
                <SvgIcon name="octicon-trash" class="ui text black job-artifacts-icon"/>
              </a>
            </li>
          </ul>
        </div>
      </div>

      <div class="action-view-right">
        <div class="job-info-header">
          <div class="job-info-header-left gt-ellipsis">
            <h3 class="job-info-header-title gt-ellipsis">
              {{ currentJob.title }}
            </h3>
            <p class="job-info-header-detail">
              {{ currentJob.detail }}
            </p>
          </div>
          <div class="job-info-header-right">
            <div class="ui top right pointing dropdown custom jump item" @click.stop="menuVisible = !menuVisible" @keyup.enter="menuVisible = !menuVisible">
              <button class="btn gt-interact-bg tw-p-2">
                <SvgIcon name="octicon-gear" :size="18"/>
              </button>
              <div class="menu transition action-job-menu" :class="{visible: menuVisible}" v-if="menuVisible" v-cloak>
                <a class="item" @click="toggleTimeDisplay('seconds')">
                  <i class="icon"><SvgIcon :name="timeVisible['log-time-seconds'] ? 'octicon-check' : 'gitea-empty-checkbox'"/></i>
                  {{ locale.showLogSeconds }}
                </a>
                <a class="item" @click="toggleTimeDisplay('stamp')">
                  <i class="icon"><SvgIcon :name="timeVisible['log-time-stamp'] ? 'octicon-check' : 'gitea-empty-checkbox'"/></i>
                  {{ locale.showTimeStamps }}
                </a>
                <a class="item" @click="toggleFullScreen()">
                  <i class="icon"><SvgIcon :name="isFullScreen ? 'octicon-check' : 'gitea-empty-checkbox'"/></i>
                  {{ locale.showFullScreen }}
                </a>
                <div class="divider"/>
                <a :class="['item', !currentJob.steps.length ? 'disabled' : '']" :href="run.link+'/jobs/'+jobIndex+'/logs'" target="_blank">
                  <i class="icon"><SvgIcon name="octicon-download"/></i>
                  {{ locale.downloadLogs }}
                </a>
              </div>
            </div>
          </div>
        </div>
        <div class="job-step-container" ref="steps" v-if="currentJob.steps.length">
          <div class="job-step-section" v-for="(jobStep, i) in currentJob.steps" :key="i">
            <div class="job-step-summary" @click.stop="isExpandable(jobStep.status) && toggleStepLogs(i)" :class="[currentJobStepsStates[i].expanded ? 'selected' : '', isExpandable(jobStep.status) && 'step-expandable']">
              <!-- If the job is done and the job step log is loaded for the first time, show the loading icon
                currentJobStepsStates[i].cursor === null means the log is loaded for the first time
              -->
              <SvgIcon v-if="isDone(run.status) && currentJobStepsStates[i].expanded && currentJobStepsStates[i].cursor === null" name="octicon-sync" class="tw-mr-2 job-status-rotate"/>
              <SvgIcon v-else :name="currentJobStepsStates[i].expanded ? 'octicon-chevron-down': 'octicon-chevron-right'" :class="['tw-mr-2', !isExpandable(jobStep.status) && 'tw-invisible']"/>
              <ActionRunStatus :status="jobStep.status" class="tw-mr-2"/>

              <span class="step-summary-msg gt-ellipsis">{{ jobStep.summary }}</span>
              <span class="step-summary-duration">{{ jobStep.duration }}</span>
            </div>

            <!-- the log elements could be a lot, do not use v-if to destroy/reconstruct the DOM,
            use native DOM elements for "log line" to improve performance, Vue is not suitable for managing so many reactive elements. -->
            <div class="job-step-logs" ref="logs" v-show="currentJobStepsStates[i].expanded"/>
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
  margin-top: 8px;
}

.action-info-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.action-info-summary-title {
  display: flex;
}

.action-info-summary-title-text {
  font-size: 20px;
  margin: 0 0 0 8px;
  flex: 1;
  overflow-wrap: anywhere;
}

.action-commit-summary {
  display: flex;
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
}

.job-artifacts-list {
  padding-left: 12px;
  list-style: none;
}

.job-artifacts-icon {
  padding-right: 3px;
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
  transition: transform 0.2s;
}

.job-brief-item .job-brief-rerun:hover {
  transform: scale(130%);
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

/* begin fomantic dropdown menu overrides */

.action-view-right .ui.dropdown .menu {
  background: var(--color-console-menu-bg);
  border-color: var(--color-console-menu-border);
}

.action-view-right .ui.dropdown .menu > .item {
  color: var(--color-console-fg);
}

.action-view-right .ui.dropdown .menu > .item:hover {
  color: var(--color-console-fg);
  background: var(--color-console-hover-bg);
}

.action-view-right .ui.dropdown .menu > .item:active {
  color: var(--color-console-fg);
  background: var(--color-console-active-bg);
}

.action-view-right .ui.dropdown .menu > .divider {
  border-top-color: var(--color-console-menu-border);
}

.action-view-right .ui.pointing.dropdown > .menu:not(.hidden)::after {
  background: var(--color-console-menu-bg);
  box-shadow: -1px -1px 0 0 var(--color-console-menu-border);
}

/* end fomantic dropdown menu overrides */

.job-info-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 12px;
  position: sticky;
  top: 0;
  height: 60px;
  z-index: 1; /* above .job-step-container */
  background: var(--color-console-bg);
  border-radius: 3px;
}

.job-info-header:has(+ .job-step-container) {
  border-radius: var(--border-radius) var(--border-radius) 0 0;
}

.job-info-header .job-info-header-title {
  color: var(--color-console-fg);
  font-size: 16px;
  margin: 0;
}

.job-info-header .job-info-header-detail {
  color: var(--color-console-fg-subtle);
  font-size: 12px;
}

.job-info-header-left {
  flex: 1;
}

.job-step-container {
  max-height: 100%;
  border-radius: 0 0 var(--border-radius) var(--border-radius);
  border-top: 1px solid var(--color-console-border);
  z-index: 0;
}

.job-step-container .job-step-summary {
  padding: 5px 10px;
  display: flex;
  align-items: center;
  border-radius: var(--border-radius);
}

.job-step-container .job-step-summary.step-expandable {
  cursor: pointer;
}

.job-step-container .job-step-summary.step-expandable:hover {
  color: var(--color-console-fg);
  background: var(--color-console-hover-bg);
}

.job-step-container .job-step-summary .step-summary-msg {
  flex: 1;
}

.job-step-container .job-step-summary .step-summary-duration {
  margin-left: 16px;
}

.job-step-container .job-step-summary.selected {
  color: var(--color-console-fg);
  background-color: var(--color-console-active-bg);
  position: sticky;
  top: 60px;
}

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

<style>
/* some elements are not managed by vue, so we need to use global style */
.job-status-rotate {
  animation: job-status-rotate-keyframes 1s linear infinite;
}

@keyframes job-status-rotate-keyframes {
  100% {
    transform: rotate(-360deg);
  }
}

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

.job-step-section .job-step-logs .job-log-line .log-msg {
  flex: 1;
  word-break: break-all;
  white-space: break-spaces;
  margin-left: 10px;
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

/* TODO: group support

.job-log-group {

}
.job-log-group-summary {

}
.job-log-list {

} */
</style>
