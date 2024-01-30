import {
  initRepoIssueWipTitle,
} from '../../features/repo-issue.js';

export function initIssueNew() {
  if (!document.querySelector('#repo-issue-new')) {
    return
  }

  initRepoIssueWipTitle();
}
