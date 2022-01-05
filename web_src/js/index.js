import './publicpath.js';

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
import {initCommentContent, initMarkupContent} from './markup/content.js';

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
  initRepoPullRequestReview,
} from './features/repo-issue.js';
import {initRepoEllipsisButton, initRepoCommitLastCommitLoader} from './features/repo-commit.js';
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

// Silence fomantic's error logging when tabs are used without a target content element
$.fn.tab.settings.silent = true;

initVueEnv();

$(document).ready(() => {
  initGlobalCommon();

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

  initUserAuthLinkAccountView();
  initUserAuthOauth2();
  initUserAuthWebAuthn();
  initUserAuthWebAuthnRegister();
  initUserSettings();
});
