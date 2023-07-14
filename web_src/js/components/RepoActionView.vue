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
        <button class="ui basic small compact button gt-mr-0" @click="rerun()" v-else-if="run.canRerun">
          {{ locale.rerun_all }}
        </button>
      </div>
      <div class="action-commit-summary">
        {{ run.commit.localeCommit }}
        <a class="muted" :href="run.commit.link">{{ run.commit.shortSHA }}</a>
        {{ run.commit.localePushedBy }}
        <a class="muted" :href="run.commit.pusher.link">{{ run.commit.pusher.displayName }}</a>
        <span class="ui label" v-if="run.commit.shortSHA">
          <a :href="run.commit.branch.link">{{ run.commit.branch.name }}</a>
        </span>
      </div>
    </div>
    <div class="action-view-body">
      <div class="action-view-left">
        <div class="job-group-section">
          <div class="job-brief-list">
            <div class="job-brief-item" :class="parseInt(jobIndex) === index ? 'selected' : ''" v-for="(job, index) in run.jobs" :key="job.id" @mouseenter="onHoverRerunIndex = job.id" @mouseleave="onHoverRerunIndex = -1">
              <a class="job-brief-link" :href="run.link+'/jobs/'+index">
                <ActionRunStatus :locale-status="locale.status[job.status]" :status="job.status"/>
                <span class="job-brief-name gt-mx-3 gt-ellipsis">{{ job.name }}</span>
              </a>
              <span class="job-brief-info">
                <SvgIcon name="octicon-sync" role="button" :data-tooltip-content="locale.rerun" class="job-brief-rerun gt-mx-3" @click="rerunJob(index)" v-if="job.canRerun && onHoverRerunIndex === job.id"/>
                <span class="step-summary-duration">{{ job.duration }}</span>
              </span>
            </div>
          </div>
        </div>
        <div class="job-artifacts" v-if="artifacts.length > 0">
          <div class="job-artifacts-title">
            {{ locale.artifactsTitle }}
          </div>
          <ul class="job-artifacts-list">
            <li class="job-artifacts-item" v-for="artifact in artifacts" :key="artifact.id">
              <a class="job-artifacts-link" target="_blank" :href="run.link+'/artifacts/'+artifact.id">
                <SvgIcon name="octicon-file" class="ui text black job-artifacts-icon"/>{{ artifact.name }}
              </a>
            </li>
          </ul>
        </div>
      </div>

      <div class="action-view-right">
        <div class="job-info-header">
          <div class="job-info-header-left">
            <h3 class="job-info-header-title">
              {{ currentJob.title }}
            </h3>
            <p class="job-info-header-detail">
              {{ currentJob.detail }}
            </p>
          </div>
          <div class="job-info-header-right">
            <div class="ui top right pointing dropdown custom jump item" @click.stop="menuVisible = !menuVisible" @keyup.enter="menuVisible = !menuVisible">
              <button class="btn gt-interact-bg gt-p-3">
                <SvgIcon name="octicon-gear" :size="18"/>
              </button>
              <div class="menu transition action-job-menu" :class="{visible: menuVisible}" v-if="menuVisible" v-cloak>
                <a class="item" :href="run.link+'/jobs/'+jobIndex+'/logs'" target="_blank">
                  <i class="icon"><SvgIcon name="octicon-download"/></i>
                  {{ locale.downloadLogs }}
                </a>
                <a class="item" @click="toggleTimeDisplay('seconds')">
                  <i class="icon"><SvgIcon v-show="timeVisible['log-time-seconds']" name="octicon-check"/></i>
                  {{ locale.showLogSeconds }}
                </a>
                <a class="item" @click="toggleTimeDisplay('stamp')">
                  <i class="icon"><SvgIcon v-show="timeVisible['log-time-stamp']" name="octicon-check"/></i>
                  {{ locale.showTimeStamps }}
                </a>
                <div class="divider"/>
                <a class="item" @click="toggleFullScreen()">
                  <i class="icon"><SvgIcon v-show="isFullScreen" name="octicon-check"/></i>
                  {{ locale.showFullScreen }}
                </a>
              </div>
            </div>
          </div>
        </div>
        <div class="job-step-container" ref="steps">
          <div class="job-step-section" v-for="(jobStep, i) in currentJob.steps" :key="i">
            <div class="job-step-summary" @click.stop="toggleStepLogs(i)" :class="currentJobStepsStates[i].expanded ? 'selected' : ''">
              <!-- If the job is done and the job step log is loaded for the first time, show the loading icon
                currentJobStepsStates[i].cursor === null means the log is loaded for the first time
              -->
              <SvgIcon v-if="isDone(run.status) && currentJobStepsStates[i].expanded && currentJobStepsStates[i].cursor === null" name="octicon-sync" class="gt-mr-3 job-status-rotate"/>
              <SvgIcon v-else :name="currentJobStepsStates[i].expanded ? 'octicon-chevron-down': 'octicon-chevron-right'" class="gt-mr-3"/>
              <ActionRunStatus :status="jobStep.status" class="gt-mr-3"/>

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

<script>
import {SvgIcon} from '../svg.js';
import ActionRunStatus from './ActionRunStatus.vue';
import {createApp} from 'vue';
import {toggleElem} from '../utils/dom.js';
import {getCurrentLocale} from '../utils.js';
import {renderAnsi} from '../render/ansi.js';

const {csrfToken} = window.config;

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
        }
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
    // rerun a job
    async rerunJob(idx) {
      const jobLink = `${this.run.link}/jobs/${idx}`;
      await this.fetchPost(`${jobLink}/rerun`);
      window.location.href = jobLink;
    },
    // rerun workflow
    async rerun() {
      await this.fetchPost(`${this.run.link}/rerun`);
      window.location.href = this.run.link;
    },
    // cancel a run
    cancelRun() {
      this.fetchPost(`${this.run.link}/cancel`);
    },
    // approve a run
    approveRun() {
      this.fetchPost(`${this.run.link}/approve`);
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
      const timeStamp = date.toLocaleString(getCurrentLocale(), {timeZoneName: 'short'});
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

    async fetchJob() {
      const logCursors = this.currentJobStepsStates.map((it, idx) => {
        // cursor is used to indicate the last position of the logs
        // it's only used by backend, frontend just reads it and passes it back, it and can be any type.
        // for example: make cursor=null means the first time to fetch logs, cursor=eof means no more logs, etc
        return {step: idx, cursor: it.cursor, expanded: it.expanded};
      });
      const resp = await this.fetchPost(
        `${this.actionsURL}/runs/${this.runIndex}/jobs/${this.jobIndex}`,
        JSON.stringify({logCursors}),
      );
      return await resp.json();
    },

    async loadJob() {
      if (this.loading) return;
      try {
        this.loading = true;

        // refresh artifacts if upload-artifact step done
        const resp = await this.fetchPost(`${this.actionsURL}/runs/${this.runIndex}/artifacts`);
        const artifacts = await resp.json();
        this.artifacts = artifacts['artifacts'] || [];

        const response = await this.fetchJob();

        // save the state to Vue data, then the UI will be updated
        this.run = response.state.run;
        this.currentJob = response.state.currentJob;

        // sync the currentJobStepsStates to store the job step states
        for (let i = 0; i < this.currentJob.steps.length; i++) {
          if (!this.currentJobStepsStates[i]) {
            // initial states for job steps
            this.currentJobStepsStates[i] = {cursor: null, expanded: false};
          }
        }
        // append logs to the UI
        for (const logs of response.logs.stepsLog) {
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


    fetchPost(url, body) {
      return fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Csrf-Token': csrfToken,
        },
        body,
      });
    },

    isDone(status) {
      return ['success', 'skipped', 'failure', 'cancelled'].includes(status);
    },

    closeDropdown() {
      if (this.menuVisible) this.menuVisible = false;
    },

    // show at most one of log seconds and timestamp (can be both invisible)
    toggleTimeDisplay(type) {
      const toToggleTypes = [];
      const other = type === 'seconds' ? 'stamp' : 'seconds';
      this.timeVisible[`log-time-${type}`] = !this.timeVisible[`log-time-${type}`];
      toToggleTypes.push(type);
      if (this.timeVisible[`log-time-${type}`] && this.timeVisible[`log-time-${other}`]) {
        this.timeVisible[`log-time-${other}`] = false;
        toToggleTypes.push(other);
      }
      for (const toToggle of toToggleTypes) {
        for (const el of this.$refs.steps.querySelectorAll(`.log-time-${toToggle}`)) {
          toggleElem(el, this.timeVisible[`log-time-${toToggle}`]);
        }
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
    }
  },
};

export default sfc;

export function initRepositoryActionView() {
  const el = document.getElementById('repo-action-view');
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
      artifactsTitle: el.getAttribute('data-locale-artifacts-title'),
      rerun_all: el.getAttribute('data-locale-rerun-all'),
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
    }
  });
  view.mount(el);
}

</script>

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
}

.action-info-summary-title {
  display: flex;
}

.action-info-summary-title-text {
  font-size: 20px;
  margin: 0 0 0 8px;
  flex: 1;
}

.action-commit-summary {
  display: flex;
  gap: 5px;
  margin: 0 0 0 28px;
}

/* ================ */
/* action view left */

.action-view-left {
  width: 30%;
  max-width: 400px;
  position: sticky;
  top: 0;
  max-height: 100vh;
  overflow-y: auto;
}

.job-group-section .job-group-summary {
  margin: 5px 0;
  padding: 10px;
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

.job-brief-item .job-brief-link {
  display: flex;
  width: 100%;
  min-width: 0;
}

.job-brief-item .job-brief-link span {
  display: flex;
  align-items: center;
}

.job-brief-item .job-brief-link .job-brief-name {
  display: block;
  width: 70%;
  color: var(--color-text);
}

.job-brief-item .job-brief-link:hover {
  text-decoration: none;
}

.job-brief-item .job-brief-info {
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

.job-info-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 12px;
  border-bottom: 1px solid var(--color-console-border);
  background-color: var(--color-console-bg);
  position: sticky;
  top: 0;
  border-radius: var(--border-radius) var(--border-radius) 0 0;
  height: 60px;
  z-index: 1;
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

.job-step-container {
  background-color: var(--color-console-bg);
  max-height: 100%;
  border-radius: 0 0 var(--border-radius) var(--border-radius);
  z-index: 0;
}

.job-step-container .job-step-summary {
  cursor: pointer;
  padding: 5px 10px;
  display: flex;
  align-items: center;
  user-select: none;
  border-radius: var(--border-radius);
}

.job-step-container .job-step-summary .step-summary-msg {
  flex: 1;
}

.job-step-container .job-step-summary .step-summary-duration {
  margin-left: 16px;
}

.job-step-container .job-step-summary:hover {
  color: var(--color-console-fg);
  background-color: var(--color-console-hover-bg);

}

.job-step-container .job-step-summary.selected {
  color: var(--color-console-fg);
  background-color: var(--color-console-active-bg);
  position: sticky;
  top: 60px;
}

@media (max-width: 768px) {
  .action-view-body {
    flex-direction: column;
  }
  .action-view-left, .action-view-right {
    width: 100%;
  }

  .action-view-left {
    max-width: none;
    overflow-y: hidden;
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
    transform: rotate(360deg);
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
  color: var(--color-grey-light);
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
  color: var(--color-grey-light);
  margin-left: 10px;
  white-space: nowrap;
}

.job-step-section .job-step-logs .job-log-line .log-msg {
  flex: 1;
  word-break: break-all;
  white-space: break-spaces;
  margin-left: 10px;
}

/* TODO: group support

.job-log-group {

}
.job-log-group-summary {

}
.job-log-list {

} */
</style>
