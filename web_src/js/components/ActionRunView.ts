import {createElementFromAttrs} from '../utils/dom.ts';
import {renderAnsi} from '../render/ansi.ts';
import {reactive} from 'vue';
import type {ActionsArtifact, ActionsJob, ActionsRun, ActionsStatus} from '../modules/gitea-actions.ts';
import type {IntervalId} from '../types.ts';
import {POST} from '../modules/fetch.ts';

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
  '##[warning]': 'warning',
  '##[notice]': 'notice',
  '##[debug]': 'debug',
  '[command]': 'command',

  // https://github.com/actions/toolkit/blob/master/docs/commands.md
  // https://github.com/actions/runner/blob/main/docs/adrs/0276-problem-matchers.md#registration
  '::add-matcher::': 'hidden',
  '##[add-matcher]': 'hidden',
  '::remove-matcher': 'hidden', // it has arguments
};

// Pattern for ::cmd:: and ::cmd args:: format (args are stripped for display)
const LogLineCmdPattern = /^::(error|warning|notice|debug)(?:\s[^:]*)?::/;

export type LogLine = {
  index: number;
  timestamp: number;
  message: string;
};

export type LogLineCommandName = 'group' | 'endgroup' | 'command' | 'error' | 'warning' | 'notice' | 'debug' | 'hidden';
export type LogLineCommand = {
  name: LogLineCommandName,
  prefix: string,
};

export function parseLogLineCommand(line: LogLine): LogLineCommand | null {
  // TODO: in the future it can be refactored to be a general parser that can parse arguments, drop the "prefix match"
  for (const prefix of Object.keys(LogLinePrefixCommandMap)) {
    if (line.message.startsWith(prefix)) {
      return {name: LogLinePrefixCommandMap[prefix], prefix};
    }
  }
  // Handle ::cmd:: and ::cmd args:: format (runner may pass these through raw)
  const match = LogLineCmdPattern.exec(line.message);
  if (match) {
    return {name: match[1] as LogLineCommandName, prefix: match[0]};
  }
  return null;
}

const LogLineLabelMap: Partial<Record<LogLineCommandName, string>> = {
  'error': 'Error',
  'warning': 'Warning',
  'notice': 'Notice',
  'debug': 'Debug',
};

export function createLogLineMessage(line: LogLine, cmd: LogLineCommand | null) {
  const logMsgAttrs = {class: 'log-msg'};
  if (cmd?.name) logMsgAttrs.class += ` log-cmd-${cmd.name}`; // make it easier to add styles to some commands like "error"

  // TODO: for some commands (::group::), the "prefix removal" works well, for some commands with "arguments" (::remove-matcher ...::),
  // it needs to do further processing in the future (fortunately, at the moment we don't need to handle these commands)
  const msgContent = cmd ? line.message.substring(cmd.prefix.length) : line.message;

  const logMsg = createElementFromAttrs('span', logMsgAttrs);
  const label = cmd ? LogLineLabelMap[cmd.name] : null;
  if (label) {
    logMsg.append(createElementFromAttrs('span', {class: 'log-msg-label'}, `${label}:`));
    const msgSpan = document.createElement('span');
    msgSpan.innerHTML = ` ${renderAnsi(msgContent.trimStart())}`;
    logMsg.append(msgSpan);
  } else {
    logMsg.innerHTML = renderAnsi(msgContent);
  }
  return logMsg;
}

export type JobGroup = {
  // The workflow job id shared by every entry. Renders as the group header
  // label when iterations.length > 1.
  jobId: string;
  iterations: ActionsJob[];
  // Aggregate status across iterations — mirrors the backend
  // AggregateJobStatus precedence (cancelling > running > waiting > blocked
  // > cancelled > failure; all-success or all-skipped collapse).
  aggregateStatus: ActionsStatus;
};

// groupJobsByMatrix walks the (already contiguous-by-jobId, thanks to the
// backend's SortMatrixGroupsByName) job list and bundles consecutive entries
// sharing a workflow jobId into a group. Single-entry groups render flat;
// multi-entry groups render a synthetic header followed by indented children.
// Matches GitHub's modern run-detail UI (one expandable header per matrix).
export function groupJobsByMatrix(jobs: ReadonlyArray<ActionsJob>): JobGroup[] {
  const groups: JobGroup[] = [];
  for (const job of jobs) {
    const last = groups[groups.length - 1];
    if (last && last.jobId === job.jobId) {
      last.iterations.push(job);
    } else {
      groups.push({jobId: job.jobId, iterations: [job], aggregateStatus: 'unknown'});
    }
  }
  for (const g of groups) {
    g.aggregateStatus = aggregateIterationStatus(g.iterations.map((j) => j.status));
  }
  return groups;
}

// aggregateIterationStatus mirrors models/actions/run_job.go AggregateJobStatus
// so the synthetic group header reflects the same precedence the backend
// would compute for an attempt-level status.
export function aggregateIterationStatus(statuses: ReadonlyArray<ActionsStatus>): ActionsStatus {
  if (statuses.length === 0) return 'unknown';
  if (statuses.every((s) => s === 'skipped')) return 'skipped';
  if (statuses.every((s) => s === 'success' || s === 'skipped')) return 'success';
  if (statuses.some((s) => s === 'cancelling')) return 'cancelling';
  if (statuses.some((s) => s === 'running')) return 'running';
  if (statuses.some((s) => s === 'waiting')) return 'waiting';
  if (statuses.some((s) => s === 'blocked')) return 'blocked';
  if (statuses.some((s) => s === 'cancelled')) return 'cancelled';
  if (statuses.some((s) => s === 'failure')) return 'failure';
  return 'unknown';
}

export function createEmptyActionsRun(): ActionsRun {
  return {
    repoId: 0,
    link: '',
    viewLink: '',
    title: '',
    titleHTML: '',
    status: '' as ActionsStatus, // do not show the status before initialized, otherwise it would show an incorrect "error" icon
    canCancel: false,
    canApprove: false,
    canRerun: false,
    canRerunFailed: false,
    canDeleteArtifact: false,
    done: false,
    workflowID: '',
    workflowLink: '',
    isSchedule: false,
    runAttempt: 0,
    attempts: [],
    duration: '',
    triggeredAt: 0,
    triggerEvent: '',
    jobs: [] as Array<ActionsJob>,
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
  };
}

export function createActionRunViewStore(viewUrl: string) {
  let loadingAbortController: AbortController | null = null;
  let intervalID: IntervalId | null = null;
  const viewData = reactive({
    currentRun: createEmptyActionsRun(),
    runArtifacts: [] as Array<ActionsArtifact>,
  });
  const loadCurrentRun = async () => {
    if (loadingAbortController) return;
    const abortController = new AbortController();
    loadingAbortController = abortController;
    try {
      const resp = await POST(viewUrl, {signal: abortController.signal, data: {}});
      const runResp = await resp.json();
      if (loadingAbortController !== abortController) return;

      viewData.runArtifacts = runResp.artifacts || [];
      viewData.currentRun = runResp.state.run;
      // clear the interval timer if the job is done
      if (viewData.currentRun.done && intervalID) {
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
  };

  return reactive({
    viewData,

    async startPollingCurrentRun() {
      await loadCurrentRun();
      intervalID = setInterval(() => loadCurrentRun(), 1000);
    },
    async forceReloadCurrentRun() {
      loadingAbortController?.abort();
      loadingAbortController = null;
      await loadCurrentRun();
    },
    stopPollingCurrentRun() {
      if (!intervalID) return;
      clearInterval(intervalID);
      intervalID = null;
    },
  });
}

export type ActionRunViewStore = ReturnType<typeof createActionRunViewStore>;
