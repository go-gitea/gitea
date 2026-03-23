// see "models/actions/status.go", if it needs to be used somewhere else, move it to a shared file like "types/actions.ts"
export type ActionsRunStatus = 'unknown' | 'waiting' | 'running' | 'success' | 'failure' | 'cancelled' | 'skipped' | 'blocked';

export type ActionsRun = {
  link: string,
  title: string,
  titleHTML: string,
  status: ActionsRunStatus,
  canCancel: boolean,
  canApprove: boolean,
  canRerun: boolean,
  canRerunFailed: boolean,
  canDeleteArtifact: boolean,
  done: boolean,
  workflowID: string,
  workflowLink: string,
  isSchedule: boolean,
  duration: string,
  triggeredAt: number,
  triggerEvent: string,
  jobs: Array<ActionsJob>,
  commit: {
    localeCommit: string,
    localePushedBy: string,
    shortSHA: string,
    link: string,
    pusher: {
      displayName: string,
      link: string,
    },
    branch: {
      name: string,
      link: string,
      isDeleted: boolean,
    },
  },
};

export type ActionsJob = {
  id: number;
  jobId: string;
  name: string;
  status: ActionsRunStatus;
  canRerun: boolean;
  needs?: string[];
  duration: string;
};

export type ActionsArtifact = {
  name: string;
  status: string;
};
