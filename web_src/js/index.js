import './publicpath.js';

import {initVueEnv} from './components/VueComponentLoader.js';
import {initRepoActivityTopAuthorsChart} from './components/RepoActivityTopAuthors.vue';
import {initDashboardRepoList} from './components/DashboardRepoList.js';

import attachTribute from './features/tribute.js';
import initGlobalCopyToClipboardListener from './features/clipboard.js';
import initContextPopups from './features/contextpopup.js';
import initGitGraph from './features/gitgraph.js';
import initHeatmap from './features/heatmap.js';
import initImageDiff from './features/imagediff.js';
import initMigration from './features/migration.js';
import initProject from './features/projects.js';
import initServiceWorker from './features/serviceworker.js';
import initTableSort from './features/tablesort.js';
import {initAdminUserListSearchForm} from './features/admin-users.js';
import {initMarkupAnchors} from './markup/anchors.js';
import {initNotificationCount, initNotificationsTable} from './features/notification.js';
import {initLastCommitLoader} from './features/lastcommitloader.js';
import {initIssueContentHistory} from './features/issue-content-history.js';
import {initStopwatch} from './features/stopwatch.js';
import {initDiffShowMore} from './features/diff.js';
import {initCommentContent, initMarkupContent} from './markup/content.js';

import {initUserAuthLinkAccountView, initUserAuthOauth2} from './features/user-auth.js';
import {
  initRepoDiffConversationForm,
  initRepoDiffFileViewToggle,
  initRepoDiffReviewButton,
} from './features/repo-diff.js';
import {
  initRepoIssueDue,
  initRepoIssueList,
  initRepoIssueReferenceRepositorySearch,
  initRepoIssueTimeTracking,
  initRepoIssueWipTitle,
  initRepoPullRequestMergeInstruction,
  initRepoPullRequestReview,
} from './features/repo-issue.js';
import {initRepoCommitButton} from './features/repo-commit.js';
import {
  initFootLanguageMenu,
  initGlobalButtonClickOnEnter,
  initGlobalButtons,
  initGlobalCommon,
  initGlobalDropzone,
  initGlobalEnterQuickSubmit,
  initGlobalFormDirtyLeaveConfirm,
  initGlobalLinkActions,
  initHeadNavbarContentToggle,
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
import {initOrgTeamSearchRepoBox, initOrgTeamSettings} from './features/org-team.js';
import {initUserAuthU2fAuth, initUserAuthU2fRegister} from './features/user-auth-u2f.js';
import {initRepoRelease, initRepoReleaseEditor} from './features/repo-release.js';
import {initRepoEditor} from './features/repo-editor.js';
import {initSearchUserBox} from './features/comp/SearchUserBox.js';
import {initInstall} from './features/install.js';
import {initWebHookEditor} from './features/comp/WebHookEditor.js';
import {initCommonIssue} from './features/common-issue.js';
import {initRepoBranchButton} from './features/repo-branch.js';
import {initCommonOrganization} from './features/common-organization.js';
import {initRepoWikiForm} from './features/repo-wiki.js';
import {initRepoCommentForm, initRepository} from './features/repo-legacy.js';

// Silence fomantic's error logging when tabs are used without a target content element
$.fn.tab.settings.silent = true;

initVueEnv();

$(document).ready(async () => {
  initGlobalCommon();
  initGlobalDropzone();
  initGlobalLinkActions();
  initGlobalButtons();
  initRepoBranchButton();

  initCommonIssue();

  initSearchUserBox();
  initRepoSettingSearchTeamBox();
  initOrgTeamSearchRepoBox();

  initGlobalButtonClickOnEnter();
  initMarkupAnchors();
  initCommentContent();
  initRepoCommentForm();
  initInstall();
  initRepoArchiveLinks();
  initRepository();
  initMigration();
  initRepoWikiForm();
  initRepoEditor();
  initCommonOrganization();
  initWebHookEditor();
  initAdminCommon();
  initRepoCodeView();
  initRepoActivityTopAuthorsChart();
  initDashboardRepoList();
  initOrgTeamSettings();
  initGlobalEnterQuickSubmit();
  initHeadNavbarContentToggle();
  initFootLanguageMenu();
  initRepoTopicBar();
  initUserAuthU2fAuth();
  initUserAuthU2fRegister();
  initRepoIssueList();
  initRepoIssueTimeTracking();
  initRepoIssueDue();
  initRepoIssueWipTitle();
  initRepoPullRequestReview();
  initRepoMigrationStatusChecker();
  initRepoTemplateSearch();
  initRepoIssueReferenceRepositorySearch();
  initContextPopups();
  initTableSort();
  initNotificationsTable();
  initLastCommitLoader();
  initRepoPullRequestMergeInstruction();
  initRepoDiffFileViewToggle();
  initRepoReleaseEditor();
  initRepoRelease();
  initDiffShowMore();
  initIssueContentHistory();
  initAdminUserListSearchForm();
  initGlobalCopyToClipboardListener();
  initUserAuthOauth2();
  initRepoDiffReviewButton();
  initRepoCommitButton();
  initAdminEmails();
  initGlobalEnterQuickSubmit();
  initSshKeyFormParser();
  initGlobalFormDirtyLeaveConfirm();
  initUserSettings();
  initRepoSettingsCollaboration();
  initUserAuthLinkAccountView();
  initRepoDiffConversationForm();

  // parallel init of async loaded features
  await Promise.all([
    attachTribute(document.querySelectorAll('#content, .emoji-input')),
    initGitGraph(),
    initHeatmap(),
    initProject(),
    initServiceWorker(),
    initNotificationCount(),
    initStopwatch(),
    initMarkupContent(),
    initRepoSettingGitHook(),
    initImageDiff(),
  ]);
});
