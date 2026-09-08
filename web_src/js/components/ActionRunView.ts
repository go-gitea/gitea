import {reactive} from 'vue';
import type {ActionsArtifact, ActionsJob, ActionsRun, ActionsStatus} from '../modules/gitea-actions.ts';
import type {IntervalId} from '../types.ts';
import {POST} from '../modules/fetch.ts';

// buildJobsByParentJobID groups jobs by their parentJobID (0 = top level).
// Useful for rendering the reusable-workflow caller/child tree in the sidebar.
export function buildJobsByParentJobID(jobs: ActionsJob[]): Map<number, ActionsJob[]> {
  const childrenByParent = new Map<number, ActionsJob[]>();
  for (const job of jobs) {
    const parentID = job.parentJobID || 0;
    const existing = childrenByParent.get(parentID);
    if (existing) {
      existing.push(job);
    } else {
      childrenByParent.set(parentID, [job]);
    }
  }
  return childrenByParent;
}

export function createEmptyActionsRun(): ActionsRun {
  return {
    repoId: 0,
    index: 0,
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
    canViewWorkflowFile: true,
    isSchedule: false,
    runAttempt: 0,
    attempts: [],
    duration: '',
    triggeredAt: 0,
    triggerEvent: '',
    pullRequest: null,
    jobs: [] as Array<ActionsJob>,
    jobSummaries: [],
    commit: {
      localeCommit: '',
      localePushedBy: '',
      shortSHA: '',
      link: '',
      pusher: {
        displayName: '',
        link: '',
        avatarLink: '',
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

  return {
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
  };
}

export type ActionRunViewStore = ReturnType<typeof createActionRunViewStore>;
