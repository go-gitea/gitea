// see "models/actions/status.go", if it needs to be used somewhere else, move it to a shared file like "types/actions.ts"
export type ActionsRunStatus = 'unknown' | 'waiting' | 'running' | 'success' | 'failure' | 'cancelled' | 'skipped' | 'blocked';

export type ActionsJob = {
  id: number;
  jobId: string;
  name: string;
  status: ActionsRunStatus;
  canRerun: boolean;
  needs?: string[];
  duration: string;
};
