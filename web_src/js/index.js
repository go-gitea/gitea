// bootstrap module must be the first one to be imported, it handles webpack lazy-loading and global errors
import './bootstrap.js';

import $ from 'jquery';
import {initVueEnv} from './components/VueComponentLoader.js';
import {initRepoActivityTopAuthorsChart} from './components/RepoActivityTopAuthors.vue';
import {initDashboardRepoList} from './components/DashboardRepoList.js';

import attachTribute from './features/tribute.js';
import initGlobalCopyToClipboardListener from './features/clipboard.js';
import initContextPopups from './features/contextpopup.js';
import initRepoGraphGit from './features/repo-graph.js';
import initHeatmap from './features/heatmap.js';
import initImageDiff from './features/imagediff.js';
import initRepoMigration from './features/repo-migration.js';
import initRepoProject from './features/repo-projects.js';
import initServiceWorker from './features/serviceworker.js';
import initTableSort from './features/tablesort.js';
import {initAdminUserListSearchForm} from './features/admin-users.js';
import {initMarkupAnchors} from './markup/anchors.js';
import {initNotificationCount, initNotificationsTable} from './features/notification.js';
import {initRepoIssueContentHistory} from './features/repo-issue-content.js';
import {initStopwatch} from './features/stopwatch.js';
import {initFindFileInRepo} from './features/repo-findfile.js';
import {initCommentContent, initMarkupContent} from './markup/content.js';
import initDiffFileTree from './features/repo-diff-filetree.js';

import {initUserAuthLinkAccountView, initUserAuthOauth2} from './features/user-auth.js';
import {
  initRepoDiffConversationForm,
  initRepoDiffFileViewToggle,
  initRepoDiffReviewButton, initRepoDiffShowMore,
} from './features/repo-diff.js';
import {
  initRepoIssueDue,
  initRepoIssueList,
  initRepoIssueReferenceRepositorySearch,
  initRepoIssueTimeTracking,
  initRepoIssueWipTitle,
  initRepoPullRequestMergeInstruction,
  initRepoPullRequestAllowMaintainerEdit,
  initRepoPullRequestReview,
} from './features/repo-issue.js';
import {
  initRepoEllipsisButton,
  initRepoCommitLastCommitLoader,
  initCommitStatuses,
} from './features/repo-commit.js';
import {
  checkAppUrl,
  initFootLanguageMenu,
  initGlobalButtonClickOnEnter,
  initGlobalButtons,
  initGlobalCommon,
  initGlobalDropzone,
  initGlobalEnterQuickSubmit,
  initGlobalFormDirtyLeaveConfirm,
  initGlobalLinkActions,
  initHeadNavbarContentToggle,
  initGlobalTooltips,
} from './features/common-global.js';
import {initRepoTopicBar} from './features/repo-home.js';
import {initAdminEmails} from './features/admin-emails.js';
import {initAdminCommon} from './features/admin-common.js';
import {initRepoTemplateSearch} from './features/repo-template.js';
import {initRepoCodeView} from './features/repo-code.js';
import {initSshKeyFormParser} from './features/sshkey-helper.js';
import {initUserSettings} from './features/user-settings.js';
import {initRepoArchiveLinks} from './features/repo-common.js';
import {initRepoMigrationStatusChecker} from './features/repo-migrate.js';
import {
  initRepoSettingGitHook,
  initRepoSettingsCollaboration,
  initRepoSettingSearchTeamBox,
} from './features/repo-settings.js';
import {initViewedCheckboxListenerFor} from './features/pull-view-file.js';
import {initOrgTeamSearchRepoBox, initOrgTeamSettings} from './features/org-team.js';
import {initUserAuthWebAuthn, initUserAuthWebAuthnRegister} from './features/user-auth-webauthn.js';
import {initRepoRelease, initRepoReleaseEditor} from './features/repo-release.js';
import {initRepoEditor} from './features/repo-editor.js';
import {initCompSearchUserBox} from './features/comp/SearchUserBox.js';
import {initInstall} from './features/install.js';
import {initCompWebHookEditor} from './features/comp/WebHookEditor.js';
import {initCommonIssue} from './features/common-issue.js';
import {initRepoBranchButton} from './features/repo-branch.js';
import {initCommonOrganization} from './features/common-organization.js';
import {initRepoWikiForm} from './features/repo-wiki.js';
import {initRepoCommentForm, initRepository} from './features/repo-legacy.js';
import {initFormattingReplacements} from './features/formatting.js';
import {initMcaptcha} from './features/mcaptcha.js';

// Run time-critical code as soon as possible. This is safe to do because this
// script appears at the end of <body> and rendered HTML is accessible at that point.
initFormattingReplacements();

// Silence fomantic's error logging when tabs are used without a target content element
$.fn.tab.settings.silent = true;
// Disable the behavior of fomantic to toggle the checkbox when you press enter on a checkbox element.
$.fn.checkbox.settings.enableEnterKey = false;

initVueEnv();
$(document).ready(() => {
  initGlobalCommon();

  initGlobalTooltips();
  initGlobalButtonClickOnEnter();
  initGlobalButtons();
  initGlobalCopyToClipboardListener();
  initGlobalDropzone();
  initGlobalEnterQuickSubmit();
  initGlobalFormDirtyLeaveConfirm();
  initGlobalLinkActions();

  attachTribute(document.querySelectorAll('#content, .emoji-input'));

  initCommonIssue();
  initCommonOrganization();

  initCompSearchUserBox();
  initCompWebHookEditor();

  initInstall();

  initHeadNavbarContentToggle();
  initFootLanguageMenu();

  initCommentContent();
  initContextPopups();
  initHeatmap();
  initImageDiff();
  initMarkupAnchors();
  initMarkupContent();
  initServiceWorker();
  initSshKeyFormParser();
  initStopwatch();
  initTableSort();
  initFindFileInRepo();

  initAdminCommon();
  initAdminEmails();
  initAdminUserListSearchForm();

  initDashboardRepoList();

  initNotificationCount();
  initNotificationsTable();

  initOrgTeamSearchRepoBox();
  initOrgTeamSettings();

  initRepoActivityTopAuthorsChart();
  initRepoArchiveLinks();
  initRepoBranchButton();
  initRepoCodeView();
  initRepoCommentForm();
  initRepoEllipsisButton();
  initRepoCommitLastCommitLoader();
  initRepoDiffConversationForm();
  initRepoDiffFileViewToggle();
  initRepoDiffReviewButton();
  initRepoDiffShowMore();
  initDiffFileTree();
  initRepoEditor();
  initRepoGraphGit();
  initRepoIssueContentHistory();
  initRepoIssueDue();
  initRepoIssueList();
  initRepoIssueReferenceRepositorySearch();
  initRepoIssueTimeTracking();
  initRepoIssueWipTitle();
  initRepoMigration();
  initRepoMigrationStatusChecker();
  initRepoProject();
  initRepoPullRequestMergeInstruction();
  initRepoPullRequestAllowMaintainerEdit();
  initRepoPullRequestReview();
  initRepoRelease();
  initRepoReleaseEditor();
  initRepoSettingGitHook();
  initRepoSettingSearchTeamBox();
  initRepoSettingsCollaboration();
  initRepoTemplateSearch();
  initRepoTopicBar();
  initRepoWikiForm();
  initRepository();

  initCommitStatuses();
  initMcaptcha();

  initUserAuthLinkAccountView();
  initUserAuthOauth2();
  initUserAuthWebAuthn();
  initUserAuthWebAuthnRegister();
  initUserSettings();
  initViewedCheckboxListenerFor();
  checkAppUrl();
});
