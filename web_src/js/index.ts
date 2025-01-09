// bootstrap module must be the first one to be imported, it handles webpack lazy-loading and global errors
import './bootstrap.ts';
import './htmx.ts';

import {initDashboardRepoList} from './components/DashboardRepoList.vue';

import {initGlobalCopyToClipboardListener} from './features/clipboard.ts';
import {initContextPopups} from './features/contextpopup.ts';
import {initRepoGraphGit} from './features/repo-graph.ts';
import {initHeatmap} from './features/heatmap.ts';
import {initImageDiff} from './features/imagediff.ts';
import {initRepoMigration} from './features/repo-migration.ts';
import {initRepoProject} from './features/repo-projects.ts';
import {initTableSort} from './features/tablesort.ts';
import {initAutoFocusEnd} from './features/autofocus-end.ts';
import {initAdminUserListSearchForm} from './features/admin/users.ts';
import {initAdminConfigs} from './features/admin/config.ts';
import {initMarkupAnchors} from './markup/anchors.ts';
import {initNotificationCount, initNotificationsTable} from './features/notification.ts';
import {initRepoIssueContentHistory} from './features/repo-issue-content.ts';
import {initStopwatch} from './features/stopwatch.ts';
import {initFindFileInRepo} from './features/repo-findfile.ts';
import {initCommentContent, initMarkupContent} from './markup/content.ts';
import {initPdfViewer} from './render/pdf.ts';

import {initUserAuthOauth2, initUserCheckAppUrl} from './features/user-auth.ts';
import {
  initRepoIssueReferenceRepositorySearch,
  initRepoIssueWipTitle,
  initRepoPullRequestMergeInstruction,
  initRepoPullRequestAllowMaintainerEdit,
  initRepoPullRequestReview, initRepoIssueSidebarList, initRepoIssueFilterItemLabel,
} from './features/repo-issue.ts';
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
import {initRepositoryActionView} from './components/RepoActionView.vue';
import {initGlobalTooltips} from './modules/tippy.ts';
import {initGiteaFomantic} from './modules/fomantic.ts';
import {initSubmitEventPolyfill, onDomReady} from './utils/dom.ts';
import {initRepoIssueList} from './features/repo-issue-list.ts';
import {initCommonIssueListQuickGoto} from './features/common-issue-list.ts';
import {initRepoContributors} from './features/contributors.ts';
import {initRepoCodeFrequency} from './features/code-frequency.ts';
import {initRepoRecentCommits} from './features/recent-commits.ts';
import {initRepoDiffCommitBranchesAndTags} from './features/repo-diff-commit.ts';
import {initDirAuto} from './modules/dirauto.ts';
import {initRepositorySearch} from './features/repo-search.ts';
import {initColorPickers} from './features/colorpicker.ts';
import {initAdminSelfCheck} from './features/admin/selfcheck.ts';
import {initOAuth2SettingsDisableCheckbox} from './features/oauth2-settings.ts';
import {initGlobalFetchAction} from './features/common-fetch-action.ts';
import {initScopedAccessTokenCategories} from './features/scoped-access-token.ts';
import {
  initFootLanguageMenu,
  initGlobalDropdown,
  initGlobalTabularMenu,
  initHeadNavbarContentToggle,
} from './features/common-page.ts';
import {
  initGlobalButtonClickOnEnter,
  initGlobalButtons,
  initGlobalDeleteButton,
} from './features/common-button.ts';
import {
  initGlobalComboMarkdownEditor,
  initGlobalEnterQuickSubmit,
  initGlobalFormDirtyLeaveConfirm,
} from './features/common-form.ts';

initGiteaFomantic();
initDirAuto();
initSubmitEventPolyfill();

function callInitFunctions(functions: (() => any)[]) {
  // Start performance trace by accessing a URL by "https://localhost/?_ui_performance_trace=1" or "https://localhost/?key=value&_ui_performance_trace=1"
  // It is a quick check, no side effect so no need to do slow URL parsing.
  const initStart = performance.now();
  if (window.location.search.includes('_ui_performance_trace=1')) {
    let results: {name: string, dur: number}[] = [];
    for (const func of functions) {
      const start = performance.now();
      func();
      results.push({name: func.name, dur: performance.now() - start});
    }
    results = results.sort((a, b) => b.dur - a.dur);
    for (let i = 0; i < 20 && i < results.length; i++) {
      // eslint-disable-next-line no-console
      console.log(`performance trace: ${results[i].name} ${results[i].dur.toFixed(3)}`);
    }
  } else {
    for (const func of functions) {
      func();
    }
  }
  const initDur = performance.now() - initStart;
  if (initDur > 500) {
    console.error(`slow init functions took ${initDur.toFixed(3)}ms`);
  }
}

onDomReady(() => {
  callInitFunctions([
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

    initCommonOrganization,
    initCommonIssueListQuickGoto,

    initCompSearchUserBox,
    initCompWebHookEditor,

    initInstall,

    initHeadNavbarContentToggle,
    initFootLanguageMenu,

    initCommentContent,
    initContextPopups,
    initHeatmap,
    initImageDiff,
    initMarkupAnchors,
    initMarkupContent,
    initSshKeyFormParser,
    initStopwatch,
    initTableSort,
    initAutoFocusEnd,
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
    initRepoIssueSidebarList,
    initRepoIssueReferenceRepositorySearch,
    initRepoIssueWipTitle,
    initRepoMigration,
    initRepoMigrationStatusChecker,
    initRepoProject,
    initRepoPullRequestMergeInstruction,
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
    initScopedAccessTokenCategories,
    initColorPickers,

    initOAuth2SettingsDisableCheckbox,
  ]);
});
