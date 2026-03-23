import type {Issue} from '../types.ts';

// the getIssueIcon/getIssueColorClass logic should be kept the same as "templates/shared/issueicon.tmpl"

export function getIssueIcon(issue: Issue) {
  if (issue.pull_request) {
    if (issue.state === 'open') {
      if (issue.pull_request.draft) {
        return 'octicon-git-pull-request-draft'; // WIP PR
      }
      return 'octicon-git-pull-request'; // Open PR
    } else if (issue.pull_request.merged) {
      return 'octicon-git-merge'; // Merged PR
    }
    return 'octicon-git-pull-request-closed'; // Closed PR
  }

  if (issue.state === 'open') {
    return 'octicon-issue-opened'; // Open Issue
  }
  return 'octicon-issue-closed'; // Closed Issue
}

export function getIssueColorClass(issue: Issue) {
  if (issue.pull_request) {
    if (issue.state === 'open') {
      if (issue.pull_request.draft) {
        return 'tw-text-text-light'; // WIP PR
      }
      return 'tw-text-green'; // Open PR
    } else if (issue.pull_request.merged) {
      return 'tw-text-purple'; // Merged PR
    }
    return 'tw-text-red'; // Closed PR
  }

  if (issue.state === 'open') {
    return 'tw-text-green'; // Open Issue
  }
  return 'tw-text-red'; // Closed Issue
}
