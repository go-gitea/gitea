import type {CommitStatusMap} from './types.ts';

// make sure this matches templates/repo/commit_status.tmpl
export const commitStatus: CommitStatusMap = {
  pending: {name: 'octicon-dot-fill', color: 'yellow'},
  success: {name: 'octicon-check', color: 'green'},
  error: {name: 'gitea-exclamation', color: 'red'},
  failure: {name: 'octicon-x', color: 'red'},
  warning: {name: 'gitea-exclamation', color: 'yellow'},
};
