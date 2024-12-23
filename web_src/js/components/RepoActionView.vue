<script lang="ts">
import {SvgIcon} from '../svg.ts';
import ActionRunStatus from './ActionRunStatus.vue';
import {createApp} from 'vue';
import {createElementFromAttrs, toggleElem} from '../utils/dom.ts';
import {formatDatetime} from '../utils/time.ts';
import {renderAnsi} from '../render/ansi.ts';
import {POST, DELETE} from '../modules/fetch.ts';

// see "models/actions/status.go", if it needs to be used somewhere else, move it to a shared file like "types/actions.ts"
type RunStatus = 'unknown' | 'waiting' | 'running' | 'success' | 'failure' | 'cancelled' | 'skipped' | 'blocked';

type LogLine = {
  index: number;
  timestamp: number;
  message: string;
};

const LogLinePrefixesGroup = ['::group::', '##[group]'];
const LogLinePrefixesEndGroup = ['::endgroup::', '##[endgroup]'];

type LogLineCommand = {
  name: 'group' | 'endgroup',
  prefix: string,
}

function parseLineCommand(line: LogLine): LogLineCommand | null {
  for (const prefix of LogLinePrefixesGroup) {
    if (line.message.startsWith(prefix)) {
      return {name: 'group', prefix};
    }
  }
  for (const prefix of LogLinePrefixesEndGroup) {
    if (line.message.startsWith(prefix)) {
      return {name: 'endgroup', prefix};
    }
  }
  return null;
}

function isLogElementInViewport(el: HTMLElement): boolean {
  const rect = el.getBoundingClientRect();
  return rect.top >= 0 && rect.bottom <= window.innerHeight; // only check height but not width
}

type LocaleStorageOptions = {
  autoScroll: boolean;
  expandRunning: boolean;
};

function getLocaleStorageOptions(): LocaleStorageOptions {
  try {
    const optsJson = localStorage.getItem('actions-view-options');
    if (optsJson) return JSON.parse(optsJson);
  } catch {}
  // if no options in localStorage, or failed to parse, return default options
  return {autoScroll: true, expandRunning: false};
}

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

  watch: {
    optionAlwaysAutoScroll() {
      this.saveLocaleStorageOptions();
    },
    optionAlwaysExpandRunning() {
      this.saveLocaleStorageOptions();
    },
  },

  data() {
    const {autoScroll, expandRunning} = getLocaleStorageOptions();
    return {
      // internal state
      loadingAbortController: null,
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
      optionAlwaysAutoScroll: autoScroll ?? false,
      optionAlwaysExpandRunning: expandRunning ?? false,

      // provided by backend
      run: {
        link: '',
        title: '',
        titleHTML: '',
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
    this.intervalID = setInterval(() => this.loadJob(), 1000);
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
    saveLocaleStorageOptions() {
      const opts: LocaleStorageOptions = {autoScroll: this.optionAlwaysAutoScroll, expandRunning: this.optionAlwaysExpandRunning};
      localStorage.setItem('actions-view-options', JSON.stringify(opts));
    },

    // get the job step logs container ('.job-step-logs')
    getJobStepLogsContainer(stepIndex: number): HTMLElement {
      return this.$refs.logs[stepIndex];
    },

    // get the active logs container element, either the `job-step-logs` or the `job-log-list` in the `job-log-group`
    getActiveLogsContainer(stepIndex: number): HTMLElement {
      const el = this.getJobStepLogsContainer(stepIndex);
      return el._stepLogsActiveContainer ?? el;
    },
    // begin a log group
    beginLogGroup(stepIndex: number, startTime: number, line: LogLine, cmd: LogLineCommand) {
      const el = this.$refs.logs[stepIndex];
      const elJobLogGroupSummary = createElementFromAttrs('summary', {class: 'job-log-group-summary'},
        this.createLogLine(stepIndex, startTime, {
          index: line.index,
          timestamp: line.timestamp,
          message: line.message.substring(cmd.prefix.length),
        }),
      );
      const elJobLogList = createElementFromAttrs('div', {class: 'job-log-list'});
      const elJobLogGroup = createElementFromAttrs('details', {class: 'job-log-group'},
        elJobLogGroupSummary,
        elJobLogList,
      );
      el.append(elJobLogGroup);
      el._stepLogsActiveContainer = elJobLogList;
    },
    // end a log group
    endLogGroup(stepIndex: number, startTime: number, line: LogLine, cmd: LogLineCommand) {
      const el = this.$refs.logs[stepIndex];
      el._stepLogsActiveContainer = null;
      el.append(this.createLogLine(stepIndex, startTime, {
        index: line.index,
        timestamp: line.timestamp,
        message: line.message.substring(cmd.prefix.length),
      }));
    },

    // show/hide the step logs for a step
    toggleStepLogs(idx: number) {
      this.currentJobStepsStates[idx].expanded = !this.currentJobStepsStates[idx].expanded;
      if (this.currentJobStepsStates[idx].expanded) {
        this.loadJobForce(); // try to load the data immediately instead of waiting for next timer interval
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

    createLogLine(stepIndex: number, startTime: number, line: LogLine) {
      const lineNum = createElementFromAttrs('a', {class: 'line-num muted', href: `#jobstep-${stepIndex}-${line.index}`},
        String(line.index),
      );

      const logTimeStamp = createElementFromAttrs('span', {class: 'log-time-stamp'},
        formatDatetime(new Date(line.timestamp * 1000)), // for "Show timestamps"
      );

      const logMsg = createElementFromAttrs('span', {class: 'log-msg'});
      logMsg.innerHTML = renderAnsi(line.message);

      const seconds = Math.floor(line.timestamp - startTime);
      const logTimeSeconds = createElementFromAttrs('span', {class: 'log-time-seconds'},
        `${seconds}s`, // for "Show seconds"
      );

      toggleElem(logTimeStamp, this.timeVisible['log-time-stamp']);
      toggleElem(logTimeSeconds, this.timeVisible['log-time-seconds']);

      return createElementFromAttrs('div', {id: `jobstep-${stepIndex}-${line.index}`, class: 'job-log-line'},
        lineNum, logTimeStamp, logMsg, logTimeSeconds,
      );
    },

    shouldAutoScroll(stepIndex: number): boolean {
      if (!this.optionAlwaysAutoScroll) return false;
      const el = this.getJobStepLogsContainer(stepIndex);
      // if the logs container is empty, then auto-scroll if the step is expanded
      if (!el.lastChild) return this.currentJobStepsStates[stepIndex].expanded;
      return isLogElementInViewport(el.lastChild);
    },

    appendLogs(stepIndex: number, startTime: number, logLines: LogLine[]) {
      for (const line of logLines) {
        const el = this.getActiveLogsContainer(stepIndex);
        const cmd = parseLineCommand(line);
        if (cmd?.name === 'group') {
          this.beginLogGroup(stepIndex, startTime, line, cmd);
          continue;
        } else if (cmd?.name === 'endgroup') {
          this.endLogGroup(stepIndex, startTime, line, cmd);
          continue;
        }
        el.append(this.createLogLine(stepIndex, startTime, line));
      }
    },

    async deleteArtifact(name: string) {
      if (!window.confirm(this.locale.confirmDeleteArtifact.replace('%s', name))) return;
      // TODO: should escape the "name"?
      await DELETE(`${this.run.link}/artifacts/${name}`);
      await this.loadJobForce();
    },

    async fetchJobData(abortController: AbortController) {
      const logCursors = this.currentJobStepsStates.map((it, idx) => {
        // cursor is used to indicate the last position of the logs
        // it's only used by backend, frontend just reads it and passes it back, it and can be any type.
        // for example: make cursor=null means the first time to fetch logs, cursor=eof means no more logs, etc
        return {step: idx, cursor: it.cursor, expanded: it.expanded};
      });
      const resp = await POST(`${this.actionsURL}/runs/${this.runIndex}/jobs/${this.jobIndex}`, {
        signal: abortController.signal,
        data: {logCursors},
      });
      return await resp.json();
    },

    async loadJobForce() {
      this.loadingAbortController?.abort();
      this.loadingAbortController = null;
      await this.loadJob();
    },

    async loadJob() {
      if (this.loadingAbortController) return;
      const abortController = new AbortController();
      this.loadingAbortController = abortController;
      try {
        const isFirstLoad = !this.run.status;
        const job = await this.fetchJobData(abortController);
        if (this.loadingAbortController !== abortController) return;

        this.artifacts = job.artifacts || [];
        this.run = job.state.run;
        this.currentJob = job.state.currentJob;

        // sync the currentJobStepsStates to store the job step states
        for (let i = 0; i < this.currentJob.steps.length; i++) {
          const expanded = isFirstLoad && this.optionAlwaysExpandRunning && this.currentJob.steps[i].status === 'running';
          if (!this.currentJobStepsStates[i]) {
            // initial states for job steps
            this.currentJobStepsStates[i] = {cursor: null, expanded};
          }
        }

        // find the step indexes that need to auto-scroll
        const autoScrollStepIndexes = new Map<number, boolean>();
        for (const logs of job.logs.stepsLog ?? []) {
          if (autoScrollStepIndexes.has(logs.step)) continue;
          autoScrollStepIndexes.set(logs.step, this.shouldAutoScroll(logs.step));
        }

        // append logs to the UI
        for (const logs of job.logs.stepsLog ?? []) {
          // save the cursor, it will be passed to backend next time
          this.currentJobStepsStates[logs.step].cursor = logs.cursor;
          this.appendLogs(logs.step, logs.started, logs.lines);
        }

        // auto-scroll to the last log line of the last step
        let autoScrollJobStepElement: HTMLElement;
        for (let stepIndex = 0; stepIndex < this.currentJob.steps.length; stepIndex++) {
          if (!autoScrollStepIndexes.get(stepIndex)) continue;
          autoScrollJobStepElement = this.getJobStepLogsContainer(stepIndex);
        }
        autoScrollJobStepElement?.lastElementChild.scrollIntoView({behavior: 'smooth', block: 'nearest'});

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

    isDone(status: RunStatus) {
      return ['success', 'skipped', 'failure', 'cancelled'].includes(status);
    },

    isExpandable(status: RunStatus) {
      return ['success', 'running', 'failure', 'cancelled'].includes(status);
    },

    closeDropdown() {
      if (this.menuVisible) this.menuVisible = false;
    },

    toggleTimeDisplay(type: string) {
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
      const contentEl = document.querySelector('.page-content');
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
  const parentFullHeight = document.querySelector<HTMLElement>('body > div.full.height');
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
      logsAlwaysAutoScroll: el.getAttribute('data-locale-logs-always-auto-scroll'),
      logsAlwaysExpandRunning: el.getAttribute('data-locale-logs-always-expand-running'),
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
          <!-- eslint-disable-next-line vue/no-v-html -->
          <h2 class="action-info-summary-title-text" v-html="run.titleHTML"/>
        </div>
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
                <a class="item" @click="optionAlwaysAutoScroll = !optionAlwaysAutoScroll">
                  <i class="icon"><SvgIcon :name="optionAlwaysAutoScroll ? 'octicon-check' : 'gitea-empty-checkbox'"/></i>
                  {{ locale.logsAlwaysAutoScroll }}
                </a>
                <a class="item" @click="optionAlwaysExpandRunning = !optionAlwaysExpandRunning">
                  <i class="icon"><SvgIcon :name="optionAlwaysExpandRunning ? 'octicon-check' : 'gitea-empty-checkbox'"/></i>
                  {{ locale.logsAlwaysExpandRunning }}
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

<style> /* eslint-disable-line vue-scoped-css/enforce-style-type */
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

.job-step-logs .job-log-line .log-msg {
  flex: 1;
  word-break: break-all;
  white-space: break-spaces;
  margin-left: 10px;
  overflow-wrap: anywhere;
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
