import {registerGlobalInitFunc} from '../modules/observer.ts';
import {
  initRepoCommentForm,
  initRepoIssueCodeCommentCancel, initRepoIssueCommentDelete,
  initRepoIssueCommentTargetHashReset, initRepoIssueReferenceIssue,
  initRepoIssueTitleEdit, initRepoIssueWipNewTitle, initRepoIssueWipToggle,
} from './repo-issue.ts';
import {initUnicodeEscapeButton} from './repo-unicode-escape.ts';
import {initRepoCloneButtons} from './repo-common.ts';
import {initRepoCitationPanel} from './citation.ts';
import {initCompReactionSelector} from './comp/ReactionSelector.ts';
import {initRepoSettings} from './repo-settings.ts';
import {hideElem, queryElemChildren, queryElems, showElem} from '../utils/dom.ts';
import {initRepoIssueCommentEdit} from './repo-issue-edit.ts';
import {initRepoMilestone} from './repo-milestone.ts';
import {initRepoNew} from './repo-new.ts';
import {createApp} from 'vue';
import RepoBranchTagSelector from '../components/RepoBranchTagSelector.vue';
import {initRepoPullMergeBox, initRepoPullRequestUpdate} from './repo-issue-pull.ts';
import {initRepoIssueSidebar} from './repo-issue-sidebar.ts';

function initRepoBranchTagSelector() {
  registerGlobalInitFunc('initRepoBranchTagSelector', (elRoot: HTMLInputElement) => {
    createApp(RepoBranchTagSelector, {elRoot}).mount(elRoot);
  });
}

export function initBranchSelectorTabs() {
  const elSelectBranches = document.querySelectorAll('.ui.dropdown.select-branch');
  for (const elSelectBranch of elSelectBranches) {
    queryElems(elSelectBranch, '.reference.column', (el) => el.addEventListener('click', () => {
      hideElem(elSelectBranch.querySelectorAll('.scrolling.reference-list-menu'));
      showElem(el.getAttribute('data-target')!);
      queryElemChildren(el.parentNode!, '.branch-tag-item', (el) => el.classList.remove('active'));
      el.classList.add('active');
    }));
  }
}

export function initRepository() {
  registerGlobalInitFunc('initRepoPullMergeBox', initRepoPullMergeBox);
  registerGlobalInitFunc('initRepoPullRequestUpdate', initRepoPullRequestUpdate);
  registerGlobalInitFunc('initRepoCitationPanel', initRepoCitationPanel);
  registerGlobalInitFunc('initRepoNew', initRepoNew);

  // TODO: need to refactor more legacy if-querySelector-based "init" code to "data-global-xxx" framework in the future.
  const pageContent = document.querySelector('.page-content.repository');
  if (!pageContent) return;

  initRepoBranchTagSelector();
  initRepoIssueSidebar();
  initRepoCommentForm();

  initRepoMilestone();

  initRepoCloneButtons();
  initRepoSettings();
  initRepoIssueWipNewTitle();

  initRepoIssueCommentEdit();
  initRepoIssueCommentDelete();
  initRepoIssueCodeCommentCancel();

  initCompReactionSelector();

  if (pageContent.matches('.page-content.repository.view.issue')) {
    initRepoIssueTitleEdit();
    initRepoIssueWipToggle();
    initRepoIssueCommentTargetHashReset();

    initRepoIssueReferenceIssue();
  }

  initUnicodeEscapeButton();
}
