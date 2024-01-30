import {
  initRepoIssueDue,
  initRepoIssueTimeTracking,
  initRepoIssueSidebarList,
} from '../../features/repo-issue.js';

export function initIssueView() {
  if (!document.querySelector('#repo-issue-view')) {
    return
  }

  initRepoIssueDue();
  initRepoIssueTimeTracking();
  initRepoIssueSidebarList();
}
