<script setup lang="ts">
import {nextTick, onBeforeUnmount, onMounted, ref, toRefs, watch} from 'vue';
import {SvgIcon} from '../svg.ts';
import ActionStatusIcon from './ActionStatusIcon.vue';
import {addDelegatedEventListener, createElementFromAttrs, toggleElem} from '../utils/dom.ts';
import {formatDatetime} from '../utils/time.ts';
import {POST} from '../modules/fetch.ts';
import type {IntervalId} from '../types.ts';
import {toggleFullScreen} from '../utils.ts';
import {localUserSettings} from '../modules/user-settings.ts';
import type {ActionsArtifact, ActionsRun, ActionsStatus} from '../modules/gitea-actions.ts';
import {
  type ActionRunViewStore,
  createLogLineMessage,
  type LogLine,
  type LogLineCommand,
  parseLogLineCommand
} from './ActionRunView.ts';

function isLogElementInViewport(el: Element, {extraViewPortHeight}={extraViewPortHeight: 0}): boolean {
  const rect = el.getBoundingClientRect();
  // only check whether bottom is in viewport, because the log element can be a log group which is usually tall
  return 0 <= rect.bottom && rect.bottom <= window.innerHeight + extraViewPortHeight;
}

type Step = {
  summary: string,
  duration: string,
  status: ActionsStatus,
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

type CurrentJob = {
  title: string;
  detail: string;
  steps: Array<Step>;
};

type JobData = {
  artifacts: Array<ActionsArtifact>;
  state: {
    run: ActionsRun;
    currentJob: CurrentJob;
  },
  logs: {
    stepsLog?: Array<{
      step: number;
      cursor: string | null;
      started: number;
      lines: LogLine[];
    }>;
  },
};

defineOptions({
  name: 'ActionRunJobView',
});

const props = defineProps<{
  store: ActionRunViewStore,
  jobId: number;
  actionsViewUrl: string;
  locale: Record<string, any>;
}>();
const store = props.store;
const {currentRun: run} = toRefs(store.viewData);

const defaultViewOptions: LocaleStorageOptions = {
  autoScroll: true,
  expandRunning: false,
  actionsLogShowSeconds: false,
  actionsLogShowTimestamps: false,
};

const savedViewOptions = localUserSettings.getJsonObject('actions-view-options', defaultViewOptions);
const {autoScroll, expandRunning, actionsLogShowSeconds, actionsLogShowTimestamps} = savedViewOptions;

// internal state
let loadingAbortController: AbortController | null = null;
let intervalID: IntervalId | null = null;

const currentJobStepsStates = ref<Array<JobStepState>>([]);
const menuVisible = ref(false);
const isFullScreen = ref(false);
const timeVisible = ref<Record<string, boolean>>({
  'log-time-stamp': actionsLogShowTimestamps,
  'log-time-seconds': actionsLogShowSeconds,
});
const optionAlwaysAutoScroll = ref(autoScroll);
const optionAlwaysExpandRunning = ref(expandRunning);
const currentJob = ref<CurrentJob>({
  title: '',
  detail: '',
  steps: [] as Array<Step>,
});
const stepsContainer = ref<HTMLElement | null>(null);
const jobStepLogs = ref<Array<StepContainerElement | undefined>>([]);

watch(optionAlwaysAutoScroll, () => {
  saveLocaleStorageOptions();
});

watch(optionAlwaysExpandRunning, () => {
  saveLocaleStorageOptions();
});

onMounted(async () => {
  // load job data and then auto-reload periodically
  // need to await first loadJob so this.currentJobStepsStates is initialized and can be used in hashChangeListener
  await loadJob();

  // auto-scroll to the bottom of the log group when it is opened
  // "toggle" event doesn't bubble, so we need to use 'click' event delegation to handle it
  addDelegatedEventListener(elStepsContainer(), 'click', 'summary.job-log-group-summary', (el, _) => {
    if (!optionAlwaysAutoScroll.value) return;
    const elJobLogGroup = el.closest('details.job-log-group') as HTMLDetailsElement;
    setTimeout(() => {
      if (elJobLogGroup.open && !isLogElementInViewport(elJobLogGroup)) {
        elJobLogGroup.scrollIntoView({behavior: 'smooth', block: 'end'});
      }
    }, 0);
  });

  intervalID = setInterval(() => void loadJob(), 1000);
  document.body.addEventListener('click', closeDropdown);
  void hashChangeListener();
  window.addEventListener('hashchange', hashChangeListener);
});

onBeforeUnmount(() => {
  document.body.removeEventListener('click', closeDropdown);
  window.removeEventListener('hashchange', hashChangeListener);
  // clear the interval timer when the component is unmounted
  // even our page is rendered once, not spa style
  if (intervalID) {
    clearInterval(intervalID);
    intervalID = null;
  }
});

function saveLocaleStorageOptions() {
  const opts: LocaleStorageOptions = {
    autoScroll: optionAlwaysAutoScroll.value,
    expandRunning: optionAlwaysExpandRunning.value,
    actionsLogShowSeconds: timeVisible.value['log-time-seconds'],
    actionsLogShowTimestamps: timeVisible.value['log-time-stamp'],
  };
  localUserSettings.setJsonObject('actions-view-options', opts);
}

// get the job step logs container ('.job-step-logs')
function getJobStepLogsContainer(stepIndex: number): StepContainerElement {
  return jobStepLogs.value[stepIndex] as StepContainerElement;
}

// get the active logs container element, either the `job-step-logs` or the `job-log-list` in the `job-log-group`
function getActiveLogsContainer(stepIndex: number): StepContainerElement {
  const el = getJobStepLogsContainer(stepIndex);
  return el._stepLogsActiveContainer ?? el;
}

// begin a log group
function beginLogGroup(stepIndex: number, startTime: number, line: LogLine, cmd: LogLineCommand) {
  const el = getJobStepLogsContainer(stepIndex);
  // Using "summary + details" is the best way to create a log group because it has built-in support for "toggle" and "accessibility".
  // And it makes users can use "Ctrl+F" to search the logs without opening all log groups.
  const elJobLogGroupSummary = createElementFromAttrs('summary', {class: 'job-log-group-summary'},
    createLogLine(stepIndex, startTime, line, cmd),
  );
  const elJobLogList = createElementFromAttrs('div', {class: 'job-log-list'});
  const elJobLogGroup = createElementFromAttrs('details', {class: 'job-log-group'},
    elJobLogGroupSummary,
    elJobLogList,
  );
  el.append(elJobLogGroup);
  el._stepLogsActiveContainer = elJobLogList;
}

// end a log group
function endLogGroup(stepIndex: number) {
  const el = getJobStepLogsContainer(stepIndex);
  el._stepLogsActiveContainer = undefined;
}

// show/hide the step logs for a step
function toggleStepLogs(idx: number) {
  currentJobStepsStates.value[idx].expanded = !currentJobStepsStates.value[idx].expanded;
  if (currentJobStepsStates.value[idx].expanded) {
    void loadJobForce(); // try to load the data immediately instead of waiting for next timer interval
  } else if (currentJob.value.steps[idx].status === 'running') {
    currentJobStepsStates.value[idx].manuallyCollapsed = true;
  }
}

function createLogLine(stepIndex: number, startTime: number, line: LogLine, cmd: LogLineCommand | null) {
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

  toggleElem(logTimeStamp, timeVisible.value['log-time-stamp']);
  toggleElem(logTimeSeconds, timeVisible.value['log-time-seconds']);

  const lineClass = cmd?.name ? `job-log-line log-line-${cmd.name}` : 'job-log-line';
  return createElementFromAttrs('div', {id: `jobstep-${stepIndex}-${line.index}`, class: lineClass},
    lineNum, logTimeStamp, logMsg, logTimeSeconds,
  );
}

function shouldAutoScroll(stepIndex: number): boolean {
  if (!optionAlwaysAutoScroll.value) return false;
  const el = getJobStepLogsContainer(stepIndex);
  // if the logs container is empty, then auto-scroll if the step is expanded
  if (!el.lastChild) return currentJobStepsStates.value[stepIndex].expanded;
  // use extraViewPortHeight to tolerate some extra "virtual view port" height (for example: the last line is partially visible)
  return isLogElementInViewport(el.lastChild as Element, {extraViewPortHeight: 5});
}

function appendLogs(stepIndex: number, startTime: number, logLines: LogLine[]) {
  for (const line of logLines) {
    const cmd = parseLogLineCommand(line);
    switch (cmd?.name) {
      case 'hidden':
        continue;
      case 'group':
        beginLogGroup(stepIndex, startTime, line, cmd);
        continue;
      case 'endgroup':
        endLogGroup(stepIndex);
        continue;
    }
    // the active logs container may change during the loop, for example: entering and leaving a group
    const el = getActiveLogsContainer(stepIndex);
    el.append(createLogLine(stepIndex, startTime, line, cmd));
  }
}

async function fetchJobData(abortController: AbortController): Promise<JobData> {
  const logCursors = currentJobStepsStates.value.map((it, idx) => {
    // cursor is used to indicate the last position of the logs
    // it's only used by backend, frontend just reads it and passes it back, it can be any type.
    // for example: make cursor=null means the first time to fetch logs, cursor=eof means no more logs, etc
    return {step: idx, cursor: it.cursor, expanded: it.expanded};
  });
  const resp = await POST(props.actionsViewUrl, {
    signal: abortController.signal,
    data: {logCursors},
  });
  return await resp.json();
}

async function loadJobForce() {
  loadingAbortController?.abort();
  loadingAbortController = null;
  await loadJob();
}

async function loadJob() {
  if (loadingAbortController) return;
  const abortController = new AbortController();
  loadingAbortController = abortController;
  try {
    const runJobResp = await fetchJobData(abortController);
    if (loadingAbortController !== abortController) return;

    // FIXME: this logic is quite hacky and dirty, it should be refactored in a better way in the future
    // Use consistent "store" operations to load/update the view data
    store.viewData.runArtifacts = runJobResp.artifacts || [];
    store.viewData.currentRun = runJobResp.state.run;

    currentJob.value = runJobResp.state.currentJob;
    const jobLogs = runJobResp.logs.stepsLog ?? [];

    // sync the currentJobStepsStates to store the job step states
    for (let i = 0; i < currentJob.value.steps.length; i++) {
      const autoExpand = optionAlwaysExpandRunning.value && currentJob.value.steps[i].status === 'running';
      if (!currentJobStepsStates.value[i]) {
        // initial states for job steps
        currentJobStepsStates.value[i] = {cursor: null, expanded: autoExpand, manuallyCollapsed: false};
      } else {
        // if the step is not manually collapsed by user, then auto-expand it if option is enabled
        if (autoExpand && !currentJobStepsStates.value[i].manuallyCollapsed) {
          currentJobStepsStates.value[i].expanded = true;
        }
      }
    }

    await nextTick();

    // find the step indexes that need to auto-scroll
    const autoScrollStepIndexes = new Map<number, boolean>();
    for (const stepLogs of jobLogs) {
      if (autoScrollStepIndexes.has(stepLogs.step)) continue;
      autoScrollStepIndexes.set(stepLogs.step, shouldAutoScroll(stepLogs.step));
    }

    // append logs to the UI
    for (const stepLogs of jobLogs) {
      // save the cursor, it will be passed to backend next time
      currentJobStepsStates.value[stepLogs.step].cursor = stepLogs.cursor;
      appendLogs(stepLogs.step, stepLogs.started, stepLogs.lines);
    }

    // auto-scroll to the last log line of the last step
    let autoScrollJobStepElement: StepContainerElement | undefined;
    for (let stepIndex = 0; stepIndex < currentJob.value.steps.length; stepIndex++) {
      if (!autoScrollStepIndexes.get(stepIndex)) continue;
      autoScrollJobStepElement = getJobStepLogsContainer(stepIndex);
    }
    const lastLogElem = autoScrollJobStepElement?.lastElementChild;
    if (lastLogElem && !isLogElementInViewport(lastLogElem)) {
      lastLogElem.scrollIntoView({behavior: 'smooth', block: 'end'});
    }

    // clear the interval timer if the job is done
    if (run.value.done && intervalID) {
      clearInterval(intervalID);
      intervalID = null;
    }
  } catch (e) {
    // avoid network error while unloading page, and ignore "abort" error
    if (e instanceof TypeError || abortController.signal.aborted) return;
    throw e;
  } finally {
    if (loadingAbortController === abortController) loadingAbortController = null;
  }
}

function isDone(status: ActionsStatus) {
  return ['success', 'skipped', 'failure', 'cancelled'].includes(status);
}

function isExpandable(status: ActionsStatus) {
  return ['success', 'running', 'failure', 'cancelled'].includes(status);
}

function closeDropdown() {
  if (menuVisible.value) menuVisible.value = false;
}

function elStepsContainer(): HTMLElement {
  return stepsContainer.value as HTMLElement;
}

function toggleTimeDisplay(type: 'seconds' | 'stamp') {
  timeVisible.value[`log-time-${type}`] = !timeVisible.value[`log-time-${type}`];
  for (const el of elStepsContainer().querySelectorAll(`.log-time-${type}`)) {
    toggleElem(el, timeVisible.value[`log-time-${type}`]);
  }
  saveLocaleStorageOptions();
}

function toggleFullScreenMode() {
  isFullScreen.value = !isFullScreen.value;
  toggleFullScreen(document.querySelector('.action-view-right')!, isFullScreen.value, '.action-view-body');
}

async function hashChangeListener() {
  const selectedLogStep = window.location.hash;
  if (!selectedLogStep) return;
  const [_, step, _line] = selectedLogStep.split('-');
  const stepNum = Number(step);
  if (!currentJobStepsStates.value[stepNum]) return;
  if (!currentJobStepsStates.value[stepNum].expanded && currentJobStepsStates.value[stepNum].cursor === null) {
    currentJobStepsStates.value[stepNum].expanded = true;
    // need to await for load job if the step log is loaded for the first time
    // so logline can be selected by querySelector
    await loadJob();
  }
  await nextTick();
  const logLine = elStepsContainer().querySelector(selectedLogStep);
  if (!logLine) return;
  logLine.querySelector<HTMLAnchorElement>('.line-num')!.click();
}
</script>
<template>
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
          <a class="item" @click="toggleFullScreenMode()">
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
    <div class="job-step-section" v-for="(jobStep, stepIdx) in currentJob.steps" :key="stepIdx">
      <div
        class="job-step-summary"
        @click.stop="isExpandable(jobStep.status) && toggleStepLogs(stepIdx)"
        :class="[currentJobStepsStates[stepIdx].expanded ? 'selected' : '', isExpandable(jobStep.status) && 'step-expandable']"
      >
        <!-- If the job is done and the job step log is loaded for the first time, show the loading icon
            currentJobStepsStates[i].cursor === null means the log is loaded for the first time
          -->
        <SvgIcon
          v-if="isDone(run.status) && currentJobStepsStates[stepIdx].expanded && currentJobStepsStates[stepIdx].cursor === null"
          name="gitea-running"
          class="tw-mr-2 rotate-clockwise"
        />
        <SvgIcon
          v-else
          :name="currentJobStepsStates[stepIdx].expanded ? 'octicon-chevron-down' : 'octicon-chevron-right'"
          :class="['tw-mr-2', !isExpandable(jobStep.status) && 'tw-invisible']"
        />
        <ActionStatusIcon :status="jobStep.status" icon-variant="circle-fill" class="tw-mr-2"/>
        <span class="step-summary-msg gt-ellipsis">{{ jobStep.summary }}</span>
        <span class="step-summary-duration">{{ jobStep.duration }}</span>
      </div>
      <!-- the log elements could be a lot, do not use v-if to destroy/reconstruct the DOM,
        use native DOM elements for "log line" to improve performance, Vue is not suitable for managing so many reactive elements. -->
      <div class="job-step-logs" :ref="(el) => jobStepLogs[stepIdx] = el as StepContainerElement" v-show="currentJobStepsStates[stepIdx].expanded"/>
    </div>
  </div>
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

.job-step-logs .log-msg a {
  color: var(--color-console-link) !important;
  text-decoration: underline;
}

.job-step-logs .job-log-line .log-cmd-command {
  color: var(--color-ansi-blue);
}

.job-step-logs .log-msg-label {
  font-weight: var(--font-weight-semibold);
}

.job-step-logs .log-line-error {
  background: var(--color-error-bg);
}

.job-step-logs .log-line-warning {
  background: var(--color-warning-bg);
}

.job-step-logs .log-line-notice {
  background: var(--color-info-bg);
}

.job-step-logs .log-line-debug {
  background: var(--color-secondary-alpha-30);
}

.job-step-logs .log-cmd-error > .log-msg-label {
  color: var(--color-error-text);
}

.job-step-logs .log-cmd-warning > .log-msg-label {
  color: var(--color-warning-text);
}

.job-step-logs .log-cmd-notice > .log-msg-label {
  color: var(--color-info-text);
}

.job-step-logs .log-cmd-debug > .log-msg-label {
  color: var(--color-violet);
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
