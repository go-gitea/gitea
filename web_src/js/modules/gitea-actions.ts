// see "models/actions/status.go", if it needs to be used somewhere else, move it to a shared file like "types/actions.ts"
export type ActionsStatus = 'unknown' | 'waiting' | 'running' | 'cancelling' | 'success' | 'failure' | 'cancelled' | 'skipped' | 'blocked';
export type ActionsArtifactStatus = 'expired' | 'completed';

export type ActionsRun = {
  repoId: number,
  index: number,
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
  pullRequest?: {
    index: string,
    link: string,
  } | null,
  jobs: Array<ActionsJob>,
  jobSummaries?: Array<ActionsJobSummary>,
  commit: {
    localeCommit: string,
    localePushedBy: string,
    shortSHA: string,
    link: string,
    pusher: {
      displayName: string,
      link: string,
      avatarLink: string,
    },
    branch: {
      name: string,
      link: string,
      isDeleted: boolean,
    },
  },
};

export type ActionsJobSummary = {
  jobId: number,
  jobName: string,
  summaryHTML: string,
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
  triggerUserAvatar: string;
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

  isReusableCaller: boolean;
  parentJobID: number; // 0 for top-level jobs.
  callUses?: string;
};

export type ActionsArtifact = {
  name: string;
  size: number;
  status: ActionsArtifactStatus;
  expiresUnix: number;
};
