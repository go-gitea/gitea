// see "models/actions/status.go", if it needs to be used somewhere else, move it to a shared file like "types/actions.ts"
export type ActionsStatus = 'unknown' | 'waiting' | 'running' | 'success' | 'failure' | 'cancelled' | 'skipped' | 'blocked';
export type ActionsArtifactStatus = 'expired' | 'completed';

export type ActionsRun = {
  repoId: number,
  link: string,
  viewLink: string,
  title: string,
  titleHTML: string,
  status: ActionsStatus,
  canCancel: boolean,
  canApprove: boolean,
  canRerun: boolean,
  canRerunFailed: boolean,
  canDeleteArtifact: boolean,
  done: boolean,
  workflowID: string,
  workflowLink: string,
  isSchedule: boolean,
  runAttempt: number,
  attempts: Array<ActionsRunAttempt>,
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

export type ActionsRunAttempt = {
  attempt: number;
  status: ActionsStatus;
  done: boolean;
  link: string;
  current: boolean;
  latest: boolean;
  triggeredAt: number;
  triggerUserName: string;
  triggerUserLink: string;
};

export type ActionsJob = {
  id: number;
  link: string;
  jobId: string;
  name: string;
  status: ActionsStatus;
  canRerun: boolean;
  needs?: string[];
  duration: string;
};

export type ActionsArtifact = {
  name: string;
  size: number;
  status: ActionsArtifactStatus;
  expiresUnix: number;
};
