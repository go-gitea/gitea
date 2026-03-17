<script lang="ts">
import {defineComponent, type PropType} from 'vue';
import {SvgIcon} from '../svg.ts';
import ActionRunStatus from './ActionRunStatus.vue';
import {addDelegatedEventListener, createElementFromAttrs, toggleElem} from '../utils/dom.ts';
import {formatDatetime} from '../utils/time.ts';
import {renderAnsi} from '../render/ansi.ts';
import {POST} from '../modules/fetch.ts';
import type {IntervalId} from '../types.ts';
import {toggleFullScreen} from '../utils.ts';
import {localUserSettings} from '../modules/user-settings.ts';
import type {ActionsRunStatus} from '../modules/gitea-actions.ts';

export type LogLine = {
  index: number;
  timestamp: number;
  message: string;
};

type LogLineCommandName = 'group' | 'endgroup' | 'command' | 'error' | 'hidden';
type LogLineCommand = {
  name: LogLineCommandName,
  prefix: string,
}

// How GitHub Actions logs work:
// * Workflow command outputs log commands like "::group::the-title", "::add-matcher::...."
// * Workflow runner parses and processes the commands to "##[group]", apply "matchers", hide secrets, etc.
// * The reported logs are the processed logs.
// HOWEVER: Gitea runner does not completely process those commands. Many works are done by the frontend at the moment.
const LogLinePrefixCommandMap: Record<string, LogLineCommandName> = {
  '::group::': 'group',
  '##[group]': 'group',
  '::endgroup::': 'endgroup',
  '##[endgroup]': 'endgroup',

  '##[error]': 'error',
  '[command]': 'command',

  // https://github.com/actions/toolkit/blob/master/docs/commands.md
  // https://github.com/actions/runner/blob/main/docs/adrs/0276-problem-matchers.md#registration
  '::add-matcher::': 'hidden',
  '##[add-matcher]': 'hidden',
  '::remove-matcher': 'hidden', // it has arguments
};

export function parseLogLineCommand(line: LogLine): LogLineCommand | null {
  // TODO: in the future it can be refactored to be a general parser that can parse arguments, drop the "prefix match"
  for (const prefix in LogLinePrefixCommandMap) {
    if (line.message.startsWith(prefix)) {
      return {name: LogLinePrefixCommandMap[prefix], prefix};
    }
  }
  return null;
}

export function createLogLineMessage(line: LogLine, cmd: LogLineCommand | null) {
  const logMsgAttrs = {class: 'log-msg'};
  if (cmd?.name) logMsgAttrs.class += ` log-cmd-${cmd?.name}`; // make it easier to add styles to some commands like "error"

  // TODO: for some commands (::group::), the "prefix removal" works well, for some commands with "arguments" (::remove-matcher ...::),
  // it needs to do further processing in the future (fortunately, at the moment we don't need to handle these commands)
  const msgContent = cmd ? line.message.substring(cmd.prefix.length) : line.message;

  const logMsg = createElementFromAttrs('span', logMsgAttrs);
  logMsg.innerHTML = renderAnsi(msgContent);
  return logMsg;
}

function isLogElementInViewport(el: Element, {extraViewPortHeight}={extraViewPortHeight: 0}): boolean {
  const rect = el.getBoundingClientRect();
  // only check whether bottom is in viewport, because the log element can be a log group which is usually tall
  return 0 <= rect.bottom && rect.bottom <= window.innerHeight + extraViewPortHeight;
}

type Step = {
  summary: string,
  duration: string,
  status: ActionsRunStatus,
}

type JobStepState = {
  cursor: string|null,
  expanded: boolean,
  manuallyCollapsed: boolean, // whether the user manually collapsed the step, used to avoid auto-expanding it again
}

type StepContainerElement = HTMLElement & {
  // To remember the last active logs container, for example: a batch of logs only starts a group but doesn't end it,
  // then the following batches of logs should still use the same group (active logs container).
  // maybe it can be refactored to decouple from the HTML element in the future.
  _stepLogsActiveContainer?: HTMLElement;
}

type LocaleStorageOptions = {
  autoScroll: boolean;
  expandRunning: boolean;
  actionsLogShowSeconds: boolean;
  actionsLogShowTimestamps: boolean;
};

export default defineComponent({
  name: 'ActionRunJobView',
  components: {
    SvgIcon,
    ActionRunStatus,
  },
  props: {
    runId: {type: Number, required: true},
    jobId: {type: Number, required: true},
    actionsURL: {type: String, required: true},
    locale: {
      type: Object as PropType<Record<string, any>>,
      required: true,
    },
    run: {
      type: Object as PropType<Record<string, any>>,
      required: true,
    },
  },
  data() {
    const defaultViewOptions: LocaleStorageOptions = {
      autoScroll: true,
      expandRunning: false,
      actionsLogShowSeconds: false,
      actionsLogShowTimestamps: false,
    };
    const {autoScroll, expandRunning, actionsLogShowSeconds, actionsLogShowTimestamps} =
      localUserSettings.getJsonObject('actions-view-options', defaultViewOptions);

    return {
      // internal state
      loadingAbortController: null as AbortController | null,
      intervalID: null as IntervalId | null,
      currentJobStepsStates: [] as Array<JobStepState>,
      menuVisible: false,
      isFullScreen: false,
      timeVisible: {
        'log-time-stamp': actionsLogShowTimestamps,
        'log-time-seconds': actionsLogShowSeconds,
      } as Record<string, boolean>,
      optionAlwaysAutoScroll: autoScroll,
      optionAlwaysExpandRunning: expandRunning,
      currentJob: {
        title: '',
        detail: '',
        steps: [
          // {
          //   summary: '',
          //   duration: '',
          //   status: '',
          // }
        ] as Array<Step>,
      },
    };
  },
  watch: {
    optionAlwaysAutoScroll() {
      this.saveLocaleStorageOptions();
    },
    optionAlwaysExpandRunning() {
      this.saveLocaleStorageOptions();
    },
  },
  async mounted() {
    // load job data and then auto-reload periodically
    // need to await first loadJob so this.currentJobStepsStates is initialized and can be used in hashChangeListener
    await this.loadJob();

    // auto-scroll to the bottom of the log group when it is opened
    // "toggle" event doesn't bubble, so we need to use 'click' event delegation to handle it
    addDelegatedEventListener(this.elStepsContainer(), 'click', 'summary.job-log-group-summary', (el, _) => {
      if (!this.optionAlwaysAutoScroll) return;
      const elJobLogGroup = el.closest('details.job-log-group') as HTMLDetailsElement;
      setTimeout(() => {
        if (elJobLogGroup.open && !isLogElementInViewport(elJobLogGroup)) {
          elJobLogGroup.scrollIntoView({behavior: 'smooth', block: 'end'});
        }
      }, 0);
    });

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
      const opts: LocaleStorageOptions = {
        autoScroll: this.optionAlwaysAutoScroll,
        expandRunning: this.optionAlwaysExpandRunning,
        actionsLogShowSeconds: this.timeVisible['log-time-seconds'],
        actionsLogShowTimestamps: this.timeVisible['log-time-stamp'],
      };
      localUserSettings.setJsonObject('actions-view-options', opts);
    },
    // get the job step logs container ('.job-step-logs')
    getJobStepLogsContainer(stepIndex: number): StepContainerElement {
      return (this.$refs.logs as any)[stepIndex];
    },
    // get the active logs container element, either the `job-step-logs` or the `job-log-list` in the `job-log-group`
    getActiveLogsContainer(stepIndex: number): StepContainerElement {
      const el = this.getJobStepLogsContainer(stepIndex);
      return el._stepLogsActiveContainer ?? el;
    },
    // begin a log group
    beginLogGroup(stepIndex: number, startTime: number, line: LogLine, cmd: LogLineCommand) {
      const el = (this.$refs.logs as any)[stepIndex] as StepContainerElement;
      const elJobLogGroupSummary = createElementFromAttrs('summary', {class: 'job-log-group-summary'},
        this.createLogLine(stepIndex, startTime, line, cmd),
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
      const el = (this.$refs.logs as any)[stepIndex];
      el._stepLogsActiveContainer = null;
      el.append(this.createLogLine(stepIndex, startTime, line, cmd));
    },
    // show/hide the step logs for a step
    toggleStepLogs(idx: number) {
      this.currentJobStepsStates[idx].expanded = !this.currentJobStepsStates[idx].expanded;
      if (this.currentJobStepsStates[idx].expanded) {
        this.loadJobForce(); // try to load the data immediately instead of waiting for next timer interval
      } else if (this.currentJob.steps[idx].status === 'running') {
        this.currentJobStepsStates[idx].manuallyCollapsed = true;
      }
    },
    createLogLine(stepIndex: number, startTime: number, line: LogLine, cmd: LogLineCommand | null) {
      const lineNum = createElementFromAttrs('a', {class: 'line-num muted', href: `#jobstep-${stepIndex}-${line.index}`},
        String(line.index),
      );
      const logTimeStamp = createElementFromAttrs('span', {class: 'log-time-stamp'},
        formatDatetime(new Date(line.timestamp * 1000)), // for "Show timestamps"
      );
      const logMsg = createLogLineMessage(line, cmd);
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
      // use extraViewPortHeight to tolerate some extra "virtual view port" height (for example: the last line is partially visible)
      return isLogElementInViewport(el.lastChild as Element, {extraViewPortHeight: 5});
    },
    appendLogs(stepIndex: number, startTime: number, logLines: LogLine[]) {
      for (const line of logLines) {
        const cmd = parseLogLineCommand(line);
        switch (cmd?.name) {
          case 'hidden':
            continue;
          case 'group':
            this.beginLogGroup(stepIndex, startTime, line, cmd);
            continue;
          case 'endgroup':
            this.endLogGroup(stepIndex, startTime, line, cmd);
            continue;
        }
        // the active logs container may change during the loop, for example: entering and leaving a group
        const el = this.getActiveLogsContainer(stepIndex);
        el.append(this.createLogLine(stepIndex, startTime, line, cmd));
      }
    },
    async fetchJobData(abortController: AbortController) {
      const logCursors = this.currentJobStepsStates.map((it, idx) => {
        // cursor is used to indicate the last position of the logs
        // it's only used by backend, frontend just reads it and passes it back, it can be any type.
        // for example: make cursor=null means the first time to fetch logs, cursor=eof means no more logs, etc
        return {step: idx, cursor: it.cursor, expanded: it.expanded};
      });
      const url = `${this.actionsURL}/runs/${this.runId}/jobs/${this.jobId}`;
      const resp = await POST(url, {
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
        const job = await this.fetchJobData(abortController);
        if (this.loadingAbortController !== abortController) return;

        this.currentJob = job.state.currentJob;

        // sync the currentJobStepsStates to store the job step states
        for (let i = 0; i < this.currentJob.steps.length; i++) {
          const autoExpand = this.optionAlwaysExpandRunning && this.currentJob.steps[i].status === 'running';
          if (!this.currentJobStepsStates[i]) {
            // initial states for job steps
            this.currentJobStepsStates[i] = {cursor: null, expanded: autoExpand,  manuallyCollapsed: false};
          } else {
            // if the step is not manually collapsed by user, then auto-expand it if option is enabled
            if (autoExpand && !this.currentJobStepsStates[i].manuallyCollapsed) {
              this.currentJobStepsStates[i].expanded = true;
            }
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
        let autoScrollJobStepElement: StepContainerElement | undefined;
        for (let stepIndex = 0; stepIndex < this.currentJob.steps.length; stepIndex++) {
          if (!autoScrollStepIndexes.get(stepIndex)) continue;
          autoScrollJobStepElement = this.getJobStepLogsContainer(stepIndex);
        }
        const lastLogElem = autoScrollJobStepElement?.lastElementChild;
        if (lastLogElem && !isLogElementInViewport(lastLogElem)) {
          lastLogElem.scrollIntoView({behavior: 'smooth', block: 'end'});
        }

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
    isDone(status: ActionsRunStatus) {
      return ['success', 'skipped', 'failure', 'cancelled'].includes(status);
    },
    isExpandable(status: ActionsRunStatus) {
      return ['success', 'running', 'failure', 'cancelled'].includes(status);
    },
    closeDropdown() {
      if (this.menuVisible) this.menuVisible = false;
    },
    elStepsContainer(): HTMLElement {
      return this.$refs.stepsContainer as HTMLElement;
    },
    toggleTimeDisplay(type: 'seconds' | 'stamp') {
      this.timeVisible[`log-time-${type}`] = !this.timeVisible[`log-time-${type}`];
      for (const el of this.elStepsContainer().querySelectorAll(`.log-time-${type}`)) {
        toggleElem(el, this.timeVisible[`log-time-${type}`]);
      }
      this.saveLocaleStorageOptions();
    },
    toggleFullScreen() {
      this.isFullScreen = !this.isFullScreen;
      toggleFullScreen('.action-view-right', this.isFullScreen, '.action-view-body');
    },
    async hashChangeListener() {
      const selectedLogStep = window.location.hash;
      if (!selectedLogStep) return;
      const [_, step, _line] = selectedLogStep.split('-');
      const stepNum = Number(step);
      if (!this.currentJobStepsStates[stepNum]) return;
      if (!this.currentJobStepsStates[stepNum].expanded && this.currentJobStepsStates[stepNum].cursor === null) {
        this.currentJobStepsStates[stepNum].expanded = true;
        // need to await for load job if the step log is loaded for the first time
        // so logline can be selected by querySelector
        await this.loadJob();
      }
      const logLine = this.elStepsContainer().querySelector(selectedLogStep);
      if (!logLine) return;
      logLine.querySelector<HTMLAnchorElement>('.line-num')!.click();
    },
  },
});
</script>
<template>
  <!-- <div> -->
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
        <button class="ui button tw-px-3">
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
          <a :class="['item', !currentJob.steps.length ? 'disabled' : '']" :href="run.link + '/jobs/' + jobId + '/logs'" download>
            <i class="icon"><SvgIcon name="octicon-download"/></i>
            {{ locale.downloadLogs }}
          </a>
        </div>
      </div>
    </div>
  </div>
  <!-- always create the node because we have our own event listeners on it, don't use "v-if" -->
  <div class="job-step-container" ref="stepsContainer" v-show="currentJob.steps.length">
    <div class="job-step-section" v-for="(jobStep, i) in currentJob.steps" :key="i">
      <div
        class="job-step-summary"
        @click.stop="isExpandable(jobStep.status) && toggleStepLogs(i)"
        :class="[currentJobStepsStates[i].expanded ? 'selected' : '', isExpandable(jobStep.status) && 'step-expandable']"
      >
        <!-- If the job is done and the job step log is loaded for the first time, show the loading icon
            currentJobStepsStates[i].cursor === null means the log is loaded for the first time
          -->
        <SvgIcon
          v-if="isDone(run.status) && currentJobStepsStates[i].expanded && currentJobStepsStates[i].cursor === null"
          name="gitea-running"
          class="tw-mr-2 rotate-clockwise"
        />
        <SvgIcon
          v-else
          :name="currentJobStepsStates[i].expanded ? 'octicon-chevron-down' : 'octicon-chevron-right'"
          :class="['tw-mr-2', !isExpandable(jobStep.status) && 'tw-invisible']"
        />
        <ActionRunStatus :status="jobStep.status" class="tw-mr-2"/>
        <span class="step-summary-msg gt-ellipsis">{{ jobStep.summary }}</span>
        <span class="step-summary-duration">{{ jobStep.duration }}</span>
      </div>
      <!-- the log elements could be a lot, do not use v-if to destroy/reconstruct the DOM,
        use native DOM elements for "log line" to improve performance, Vue is not suitable for managing so many reactive elements. -->
      <div class="job-step-logs" ref="logs" v-show="currentJobStepsStates[i].expanded"/>
    </div>
  </div>
  <!-- </div> -->
</template>
<style scoped>
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
  /* workaround ansi_up issue related to faintStyle generating a CSS stacking context via `opacity`
     inline style which caused such elements to render above the .job-step-summary header. */
  z-index: 1;
}
</style>
