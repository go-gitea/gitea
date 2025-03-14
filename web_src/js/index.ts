// bootstrap module must be the first one to be imported, it handles webpack lazy-loading and global errors
import './bootstrap.ts';
import './htmx.ts';

import {initDashboardRepoList} from './features/dashboard.ts';
import {initGlobalCopyToClipboardListener} from './features/clipboard.ts';
import {initContextPopups} from './features/contextpopup.ts';
import {initRepoGraphGit} from './features/repo-graph.ts';
import {initHeatmap} from './features/heatmap.ts';
import {initImageDiff} from './features/imagediff.ts';
import {initRepoMigration} from './features/repo-migration.ts';
import {initRepoProject} from './features/repo-projects.ts';
import {initTableSort} from './features/tablesort.ts';
import {initAdminUserListSearchForm} from './features/admin/users.ts';
import {initAdminConfigs} from './features/admin/config.ts';
import {initMarkupAnchors} from './markup/anchors.ts';
import {initNotificationCount, initNotificationsTable} from './features/notification.ts';
import {initRepoIssueContentHistory} from './features/repo-issue-content.ts';
import {initStopwatch} from './features/stopwatch.ts';
import {initFindFileInRepo} from './features/repo-findfile.ts';
import {initMarkupContent} from './markup/content.ts';
import {initPdfViewer} from './render/pdf.ts';
import {initUserAuthOauth2, initUserCheckAppUrl} from './features/user-auth.ts';
import {initRepoPullRequestAllowMaintainerEdit, initRepoPullRequestReview, initRepoIssueSidebarDependency, initRepoIssueFilterItemLabel} from './features/repo-issue.ts';
import {initRepoEllipsisButton, initCommitStatuses} from './features/repo-commit.ts';
import {initRepoTopicBar} from './features/repo-home.ts';
import {initAdminCommon} from './features/admin/common.ts';
import {initRepoCodeView} from './features/repo-code.ts';
import {initSshKeyFormParser} from './features/sshkey-helper.ts';
import {initUserSettings} from './features/user-settings.ts';
import {initRepoActivityTopAuthorsChart, initRepoArchiveLinks} from './features/repo-common.ts';
import {initRepoMigrationStatusChecker} from './features/repo-migrate.ts';
import {initRepoDiffView} from './features/repo-diff.ts';
import {initOrgTeam} from './features/org-team.ts';
import {initUserAuthWebAuthn, initUserAuthWebAuthnRegister} from './features/user-auth-webauthn.ts';
import {initRepoRelease, initRepoReleaseNew} from './features/repo-release.ts';
import {initRepoEditor} from './features/repo-editor.ts';
import {initCompSearchUserBox} from './features/comp/SearchUserBox.ts';
import {initInstall} from './features/install.ts';
import {initCompWebHookEditor} from './features/comp/WebHookEditor.ts';
import {initRepoBranchButton} from './features/repo-branch.ts';
import {initCommonOrganization} from './features/common-organization.ts';
import {initRepoWikiForm} from './features/repo-wiki.ts';
import {initRepository, initBranchSelectorTabs} from './features/repo-legacy.ts';
import {initCopyContent} from './features/copycontent.ts';
import {initCaptcha} from './features/captcha.ts';
import {initRepositoryActionView} from './features/repo-actions.ts';
import {initGlobalTooltips} from './modules/tippy.ts';
import {initGiteaFomantic} from './modules/fomantic.ts';
import {initSubmitEventPolyfill, onDomReady} from './utils/dom.ts';
import {initRepoIssueList} from './features/repo-issue-list.ts';
import {initCommonIssueListQuickGoto} from './features/common-issue-list.ts';
import {initRepoContributors} from './features/contributors.ts';
import {initRepoCodeFrequency} from './features/code-frequency.ts';
import {initRepoRecentCommits} from './features/recent-commits.ts';
import {initRepoDiffCommitBranchesAndTags} from './features/repo-diff-commit.ts';
import {initGlobalSelectorObserver} from './modules/observer.ts';
import {initRepositorySearch} from './features/repo-search.ts';
import {initColorPickers} from './features/colorpicker.ts';
import {initAdminSelfCheck} from './features/admin/selfcheck.ts';
import {initOAuth2SettingsDisableCheckbox} from './features/oauth2-settings.ts';
import {initGlobalFetchAction} from './features/common-fetch-action.ts';
import {initFootLanguageMenu, initGlobalDropdown, initGlobalInput, initGlobalTabularMenu, initHeadNavbarContentToggle} from './features/common-page.ts';
import {initGlobalButtonClickOnEnter, initGlobalButtons, initGlobalDeleteButton} from './features/common-button.ts';
import {initGlobalComboMarkdownEditor, initGlobalEnterQuickSubmit, initGlobalFormDirtyLeaveConfirm} from './features/common-form.ts';
import {callInitFunctions} from './modules/init.ts';

initGiteaFomantic();
initSubmitEventPolyfill();

onDomReady(() => {
  const initStartTime = performance.now();
  const initPerformanceTracer = callInitFunctions([
    initGlobalDropdown,
    initGlobalTabularMenu,
    initGlobalFetchAction,
    initGlobalTooltips,
    initGlobalButtonClickOnEnter,
    initGlobalButtons,
    initGlobalCopyToClipboardListener,
    initGlobalEnterQuickSubmit,
    initGlobalFormDirtyLeaveConfirm,
    initGlobalComboMarkdownEditor,
    initGlobalDeleteButton,
    initGlobalInput,

    initCommonOrganization,
    initCommonIssueListQuickGoto,

    initCompSearchUserBox,
    initCompWebHookEditor,

    initInstall,

    initHeadNavbarContentToggle,
    initFootLanguageMenu,

    initContextPopups,
    initHeatmap,
    initImageDiff,
    initMarkupAnchors,
    initMarkupContent,
    initSshKeyFormParser,
    initStopwatch,
    initTableSort,
    initFindFileInRepo,
    initCopyContent,

    initAdminCommon,
    initAdminUserListSearchForm,
    initAdminConfigs,
    initAdminSelfCheck,

    initDashboardRepoList,

    initNotificationCount,
    initNotificationsTable,

    initOrgTeam,

    initRepoActivityTopAuthorsChart,
    initRepoArchiveLinks,
    initRepoBranchButton,
    initRepoCodeView,
    initBranchSelectorTabs,
    initRepoEllipsisButton,
    initRepoDiffCommitBranchesAndTags,
    initRepoEditor,
    initRepoGraphGit,
    initRepoIssueContentHistory,
    initRepoIssueList,
    initRepoIssueFilterItemLabel,
    initRepoIssueSidebarDependency,
    initRepoMigration,
    initRepoMigrationStatusChecker,
    initRepoProject,
    initRepoPullRequestAllowMaintainerEdit,
    initRepoPullRequestReview,
    initRepoRelease,
    initRepoReleaseNew,
    initRepoTopicBar,
    initRepoWikiForm,
    initRepository,
    initRepositoryActionView,
    initRepositorySearch,
    initRepoContributors,
    initRepoCodeFrequency,
    initRepoRecentCommits,

    initCommitStatuses,
    initCaptcha,

    initUserCheckAppUrl,
    initUserAuthOauth2,
    initUserAuthWebAuthn,
    initUserAuthWebAuthnRegister,
    initUserSettings,
    initRepoDiffView,
    initPdfViewer,
    initColorPickers,

    initOAuth2SettingsDisableCheckbox,
  ]);

  // it must be the last one, then the "querySelectorAll" only needs to be executed once for global init functions.
  initGlobalSelectorObserver(initPerformanceTracer);
  if (initPerformanceTracer) initPerformanceTracer.printResults();

  const initDur = performance.now() - initStartTime;
  if (initDur > 500) {
    console.error(`slow init functions took ${initDur.toFixed(3)}ms`);
  }
});
