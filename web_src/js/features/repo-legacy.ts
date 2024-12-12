import $ from 'jquery';
import {
  initRepoCommentFormAndSidebar,
  initRepoIssueBranchSelect, initRepoIssueCodeCommentCancel, initRepoIssueCommentDelete,
  initRepoIssueComments, initRepoIssueDependencyDelete, initRepoIssueReferenceIssue,
  initRepoIssueTitleEdit, initRepoIssueWipToggle,
  initRepoPullRequestUpdate,
} from './repo-issue.ts';
import {initUnicodeEscapeButton} from './repo-unicode-escape.ts';
import {initRepoBranchTagSelector} from '../components/RepoBranchTagSelector.vue';
import {initRepoCloneButtons} from './repo-common.ts';
import {initCitationFileCopyContent} from './citation.ts';
import {initCompLabelEdit} from './comp/LabelEdit.ts';
import {initRepoDiffConversationNav} from './repo-diff.ts';
import {initCompReactionSelector} from './comp/ReactionSelector.ts';
import {initRepoSettings} from './repo-settings.ts';
import {initRepoPullRequestMergeForm} from './repo-issue-pr-form.ts';
import {initRepoPullRequestCommitStatus} from './repo-issue-pr-status.ts';
import {hideElem, queryElemChildren, showElem} from '../utils/dom.ts';
import {initRepoIssueCommentEdit} from './repo-issue-edit.ts';
import {initRepoMilestone} from './repo-milestone.ts';
import {initRepoNew} from './repo-new.ts';

export function initBranchSelectorTabs() {
  const elSelectBranch = document.querySelector('.ui.dropdown.select-branch');
  if (!elSelectBranch) return;

  $(elSelectBranch).find('.reference.column').on('click', function () {
    hideElem($(elSelectBranch).find('.scrolling.reference-list-menu'));
    showElem(this.getAttribute('data-target'));
    queryElemChildren(this.parentNode, '.branch-tag-item', (el) => el.classList.remove('active'));
    this.classList.add('active');
    return false;
  });
}

function initRepoCommonBranchOrTagDropdown(selector: string) {
  $(selector).each(function () {
    const $dropdown = $(this);
    $dropdown.find('.reference.column').on('click', function () {
      hideElem($dropdown.find('.scrolling.reference-list-menu'));
      showElem($($(this).data('target')));
      return false;
    });
  });
}

function initRepoCommonFilterSearchDropdown(selector: string) {
  const $dropdown = $(selector);
  if (!$dropdown.length) return;

  $dropdown.dropdown({
    fullTextSearch: 'exact',
    selectOnKeydown: false,
    onChange(_text, _value, $choice) {
      if ($choice[0].getAttribute('data-url')) {
        window.location.href = $choice[0].getAttribute('data-url');
      }
    },
    message: {noResults: $dropdown[0].getAttribute('data-no-results')},
  });
}

export function initRepository() {
  if (!$('.page-content.repository').length) return;

  initRepoBranchTagSelector('.js-branch-tag-selector');
  initRepoCommentFormAndSidebar();

  // Labels
  initCompLabelEdit('.page-content.repository.labels');
  initRepoMilestone();
  initRepoNew();

  // Compare or pull request
  const $repoDiff = $('.repository.diff');
  if ($repoDiff.length) {
    initRepoCommonBranchOrTagDropdown('.choose.branch .dropdown');
    initRepoCommonFilterSearchDropdown('.choose.branch .dropdown');
  }

  initRepoCloneButtons();
  initCitationFileCopyContent();
  initRepoSettings();

  // Issues
  if ($('.repository.view.issue').length > 0) {
    initRepoIssueCommentEdit();

    initRepoIssueBranchSelect();
    initRepoIssueTitleEdit();
    initRepoIssueWipToggle();
    initRepoIssueComments();

    initRepoDiffConversationNav();
    initRepoIssueReferenceIssue();

    initRepoIssueCommentDelete();
    initRepoIssueDependencyDelete();
    initRepoIssueCodeCommentCancel();
    initRepoPullRequestUpdate();
    initCompReactionSelector();

    initRepoPullRequestMergeForm();
    initRepoPullRequestCommitStatus();
  }

  // Pull request
  const $repoComparePull = $('.repository.compare.pull');
  if ($repoComparePull.length > 0) {
    // show pull request form
    $repoComparePull.find('button.show-form').on('click', function (e) {
      e.preventDefault();
      hideElem($(this).parent());

      const $form = $repoComparePull.find('.pullrequest-form');
      showElem($form);
    });
  }

  initUnicodeEscapeButton();
}
