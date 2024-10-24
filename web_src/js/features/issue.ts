export type Issue = {id: number; title: string; state: 'open' | 'closed'; pull_request?: {draft: boolean; merged: boolean}};

export function getIssueIcon(issue: Issue) {
  if (issue.pull_request) {
    if (issue.state === 'open') {
      if (issue.pull_request.draft === true) {
        return 'octicon-git-pull-request-draft'; // WIP PR
      }
      return 'octicon-git-pull-request'; // Open PR
    } else if (issue.pull_request.merged === true) {
      return 'octicon-git-merge'; // Merged PR
    }
    return 'octicon-git-pull-request'; // Closed PR
  } else if (issue.state === 'open') {
    return 'octicon-issue-opened'; // Open Issue
  }
  return 'octicon-issue-closed'; // Closed Issue
}

export function getIssueColor(issue: Issue) {
  if (issue.pull_request) {
    if (issue.pull_request.draft === true) {
      return 'grey'; // WIP PR
    } else if (issue.pull_request.merged === true) {
      return 'purple'; // Merged PR
    }
  }
  if (issue.state === 'open') {
    return 'green'; // Open Issue
  }
  return 'red'; // Closed Issue
}
