import {registerGlobalInitFunc} from '../modules/observer.ts';
import {
  initRepoCommentFormAndSidebar,
  initRepoIssueBranchSelect, initRepoIssueCodeCommentCancel, initRepoIssueCommentDelete,
  initRepoIssueComments, initRepoIssueReferenceIssue,
  initRepoIssueTitleEdit, initRepoIssueWipNewTitle, initRepoIssueWipToggle,
  initRepoPullRequestUpdate,
} from './repo-issue.ts';
import {initUnicodeEscapeButton} from './repo-unicode-escape.ts';
import {initRepoCloneButtons} from './repo-common.ts';
import {initCitationFileCopyContent} from './citation.ts';
import {initCompLabelEdit} from './comp/LabelEdit.ts';
import {initCompReactionSelector} from './comp/ReactionSelector.ts';
import {initRepoSettings} from './repo-settings.ts';
import {initRepoPullRequestMergeForm} from './repo-issue-pr-form.ts';
import {initRepoPullRequestCommitStatus} from './repo-issue-pr-status.ts';
import {hideElem, queryElemChildren, queryElems, showElem} from '../utils/dom.ts';
import {initRepoIssueCommentEdit} from './repo-issue-edit.ts';
import {initRepoMilestone} from './repo-milestone.ts';
import {initRepoNew} from './repo-new.ts';
import {createApp} from 'vue';
import RepoBranchTagSelector from '../components/RepoBranchTagSelector.vue';

function initRepoBranchTagSelector() {
  registerGlobalInitFunc('initRepoBranchTagSelector', async (elRoot: HTMLInputElement) => {
    createApp(RepoBranchTagSelector, {elRoot}).mount(elRoot);
  });
}

export function initBranchSelectorTabs() {
  const elSelectBranches = document.querySelectorAll('.ui.dropdown.select-branch');
  for (const elSelectBranch of elSelectBranches) {
    queryElems(elSelectBranch, '.reference.column', (el) => el.addEventListener('click', () => {
      hideElem(elSelectBranch.querySelectorAll('.scrolling.reference-list-menu'));
      showElem(el.getAttribute('data-target'));
      queryElemChildren(el.parentNode, '.branch-tag-item', (el) => el.classList.remove('active'));
      el.classList.add('active');
    }));
  }
}

export function initRepository() {
  const pageContent = document.querySelector('.page-content.repository');
  if (!pageContent) return;

  initRepoBranchTagSelector();
  initRepoCommentFormAndSidebar();

  // Labels
  initCompLabelEdit('.page-content.repository.labels');
  initRepoMilestone();
  initRepoNew();

  initRepoCloneButtons();
  initCitationFileCopyContent();
  initRepoSettings();
  initRepoIssueWipNewTitle();

  // Issues
  if (pageContent.matches('.page-content.repository.view.issue')) {
    initRepoIssueCommentEdit();

    initRepoIssueBranchSelect();
    initRepoIssueTitleEdit();
    initRepoIssueWipToggle();
    initRepoIssueComments();

    initRepoIssueReferenceIssue();

    initRepoIssueCommentDelete();
    initRepoIssueCodeCommentCancel();
    initRepoPullRequestUpdate();
    initCompReactionSelector();

    initRepoPullRequestMergeForm();
    initRepoPullRequestCommitStatus();
  }

  initUnicodeEscapeButton();
}
