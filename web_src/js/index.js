// bootstrap module must be the first one to be imported, it handles webpack lazy-loading and global errors
import './bootstrap.js';

import {initRepoActivityTopAuthorsChart} from './components/RepoActivityTopAuthors.vue';
import {initDashboardRepoList} from './components/DashboardRepoList.vue';

import {initGlobalCopyToClipboardListener} from './features/clipboard.js';
import {initContextPopups} from './features/contextpopup.js';
import {initRepoGraphGit} from './features/repo-graph.js';
import {initHeatmap} from './features/heatmap.js';
import {initImageDiff} from './features/imagediff.js';
import {initRepoMigration} from './features/repo-migration.js';
import {initRepoProject} from './features/repo-projects.js';
import {initServiceWorker} from './features/serviceworker.js';
import {initTableSort} from './features/tablesort.js';
import {initAdminUserListSearchForm} from './features/admin/users.js';
import {initAdminConfigs} from './features/admin/config.js';
import {initMarkupAnchors} from './markup/anchors.js';
import {initNotificationCount, initNotificationsTable} from './features/notification.js';
import {initRepoIssueContentHistory} from './features/repo-issue-content.js';
import {initStopwatch} from './features/stopwatch.js';
import {initFindFileInRepo} from './features/repo-findfile.js';
import {initCommentContent, initMarkupContent} from './markup/content.js';

import {initUserAuthLinkAccountView, initUserAuthOauth2} from './features/user-auth.js';
import {
  initRepoIssueDue,
  initRepoIssueReferenceRepositorySearch,
  initRepoIssueTimeTracking,
  initRepoIssueWipTitle,
  initRepoPullRequestMergeInstruction,
  initRepoPullRequestAllowMaintainerEdit,
  initRepoPullRequestReview, initRepoIssueSidebarList, initRepoIssueGotoID
} from './features/repo-issue.js';
import {
  initRepoEllipsisButton,
  initRepoCommitLastCommitLoader,
  initCommitStatuses,
} from './features/repo-commit.js';
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
import {initAdminEmails} from './features/admin/emails.js';
import {initAdminCommon} from './features/admin/common.js';
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
import {initRepoDiffView} from './features/repo-diff.js';
import {initOrgTeamSearchRepoBox, initOrgTeamSettings} from './features/org-team.js';
import {initUserAuthWebAuthn, initUserAuthWebAuthnRegister} from './features/user-auth-webauthn.js';
import {initRepoRelease, initRepoReleaseNew} from './features/repo-release.js';
import {initRepoEditor} from './features/repo-editor.js';
import {initCompSearchUserBox} from './features/comp/SearchUserBox.js';
import {initInstall} from './features/install.js';
import {initCompWebHookEditor} from './features/comp/WebHookEditor.js';
import {initRepoBranchButton} from './features/repo-branch.js';
import {initCommonOrganization} from './features/common-organization.js';
import {initRepoWikiForm} from './features/repo-wiki.js';
import {initRepoCommentForm, initRepository} from './features/repo-legacy.js';
import {initCopyContent} from './features/copycontent.js';
import {initCaptcha} from './features/captcha.js';
import {initRepositoryActionView} from './components/RepoActionView.vue';
import {initGlobalTooltips} from './modules/tippy.js';
import {initGiteaFomantic} from './modules/fomantic.js';
import {onDomReady} from './utils/dom.js';
import {initRepoIssueList} from './features/repo-issue-list.js';

// Init Gitea's Fomantic settings
initGiteaFomantic();

onDomReady(() => {
  initGlobalCommon();

  initGlobalTooltips();
  initGlobalButtonClickOnEnter();
  initGlobalButtons();
  initGlobalCopyToClipboardListener();
  initGlobalDropzone();
  initGlobalEnterQuickSubmit();
  initGlobalFormDirtyLeaveConfirm();
  initGlobalLinkActions();

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
  initCopyContent();

  initAdminCommon();
  initAdminEmails();
  initAdminUserListSearchForm();
  initAdminConfigs();

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
  initRepoEditor();
  initRepoGraphGit();
  initRepoIssueContentHistory();
  initRepoIssueDue();
  initRepoIssueList();
  initRepoIssueSidebarList();
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
  initRepoReleaseNew();
  initRepoSettingGitHook();
  initRepoSettingSearchTeamBox();
  initRepoSettingsCollaboration();
  initRepoTemplateSearch();
  initRepoTopicBar();
  initRepoWikiForm();
  initRepository();
  initRepositoryActionView();

  initCommitStatuses();
  initCaptcha();

  initUserAuthLinkAccountView();
  initUserAuthOauth2();
  initUserAuthWebAuthn();
  initUserAuthWebAuthnRegister();
  initUserSettings();
  initRepoDiffView();
  initRepoIssueGotoID();
});
