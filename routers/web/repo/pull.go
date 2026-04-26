// Copyright 2018 The Gitea Authors.
// Copyright 2014 The Gogs Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/fileicon"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/glob"
	"code.gitea.io/gitea/modules/graceful"
	issue_template "code.gitea.io/gitea/modules/issue/template"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	actions_service "code.gitea.io/gitea/services/actions"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/automerge"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/forms"
	git_service "code.gitea.io/gitea/services/git"
	"code.gitea.io/gitea/services/gitdiff"
	notify_service "code.gitea.io/gitea/services/notify"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
	user_service "code.gitea.io/gitea/services/user"
)

const (
	tplCompareDiff templates.TplName = "repo/diff/compare"
	tplPullCommits templates.TplName = "repo/pulls/commits"
	tplPullFiles   templates.TplName = "repo/pulls/files"

	pullRequestTemplateKey = "PullRequestTemplate"
)

var pullRequestTemplateCandidates = []string{
	"PULL_REQUEST_TEMPLATE.md",
	"PULL_REQUEST_TEMPLATE.yaml",
	"PULL_REQUEST_TEMPLATE.yml",
	"pull_request_template.md",
	"pull_request_template.yaml",
	"pull_request_template.yml",
	".gitea/PULL_REQUEST_TEMPLATE.md",
	".gitea/PULL_REQUEST_TEMPLATE.yaml",
	".gitea/PULL_REQUEST_TEMPLATE.yml",
	".gitea/pull_request_template.md",
	".gitea/pull_request_template.yaml",
	".gitea/pull_request_template.yml",
	".github/PULL_REQUEST_TEMPLATE.md",
	".github/PULL_REQUEST_TEMPLATE.yaml",
	".github/PULL_REQUEST_TEMPLATE.yml",
	".github/pull_request_template.md",
	".github/pull_request_template.yaml",
	".github/pull_request_template.yml",
}

func getRepository(ctx *context.Context, repoID int64) *repo_model.Repository {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetRepositoryByID", err)
		}
		return nil
	}

	perm, err := access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetDoerRepoPermission", err)
		return nil
	}

	if !perm.CanRead(unit.TypeCode) {
		log.Trace("Permission Denied: User %-v cannot read %-v of repo %-v\n"+
			"User in repo has Permissions: %-+v",
			ctx.Doer,
			unit.TypeCode,
			ctx.Repo,
			perm)
		ctx.NotFound(nil)
		return nil
	}
	return repo
}

func getPullInfo(ctx *context.Context) (issue *issues_model.Issue, ok bool) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return nil, false
	}
	if err = issue.LoadPoster(ctx); err != nil {
		ctx.ServerError("LoadPoster", err)
		return nil, false
	}
	if err := issue.LoadRepo(ctx); err != nil {
		ctx.ServerError("LoadRepo", err)
		return nil, false
	}
	ctx.Data["Title"] = fmt.Sprintf("#%d - %s", issue.Index, emoji.ReplaceAliases(issue.Title))
	ctx.Data["Issue"] = issue

	if !issue.IsPull {
		ctx.Redirect(issue.Link())
		return nil, false
	}

	if err = issue.LoadPullRequest(ctx); err != nil {
		ctx.ServerError("LoadPullRequest", err)
		return nil, false
	}

	if err = issue.PullRequest.LoadHeadRepo(ctx); err != nil {
		ctx.ServerError("LoadHeadRepo", err)
		return nil, false
	}

	if ctx.IsSigned {
		// Update issue-user.
		if err = activities_model.SetIssueReadBy(ctx, issue.ID, ctx.Doer.ID); err != nil {
			ctx.ServerError("ReadBy", err)
			return nil, false
		}
	}

	return issue, true
}

func (prInfo *pullRequestViewInfo) setTemplateDataMergeTarget(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	if ctx.Repo.Owner.Name == pull.MustHeadUserName(ctx) {
		ctx.Data["HeadTarget"] = pull.HeadBranch
	} else if pull.HeadRepo == nil {
		ctx.Data["HeadTarget"] = ctx.Locale.Tr("repo.pull.deleted_branch", pull.HeadBranch)
	} else {
		ctx.Data["HeadTarget"] = pull.MustHeadUserName(ctx) + "/" + pull.HeadRepo.Name + ":" + pull.HeadBranch
	}
	ctx.Data["BaseTarget"] = pull.BaseBranch
	headBranchLink := ""
	if pull.Flow == issues_model.PullRequestFlowGithub {
		b, err := git_model.GetBranch(ctx, pull.HeadRepoID, pull.HeadBranch)
		switch {
		case err == nil:
			if !b.IsDeleted {
				headBranchLink = pull.GetHeadBranchLink(ctx)
			}
		case !git_model.IsErrBranchNotExist(err):
			log.Error("GetBranch: %v", err)
		}
	}
	ctx.Data["HeadBranchLink"] = headBranchLink
	ctx.Data["BaseBranchLink"] = pull.GetBaseBranchLink(ctx)
}

// GetPullDiffStats get Pull Requests diff stats
func GetPullDiffStats(ctx *context.Context) {
	// FIXME: this getPullInfo seems to be a duplicate call with other route handlers
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest

	mergeBaseCommitID := GetMergedBaseCommitID(ctx, issue)
	if mergeBaseCommitID == "" {
		return // no merge base, do nothing, do not stop the route handler, see below
	}

	// do not report 500 server error to end users if error occurs, otherwise a PR missing ref won't be able to view.
	headCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(pull.GetGitHeadRefName())
	if err != nil {
		log.Error("Failed to GetRefCommitID: %v, repo: %v", err, ctx.Repo.Repository.FullName())
		return
	}
	diffShortStat, err := gitdiff.GetDiffShortStat(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, mergeBaseCommitID, headCommitID)
	if err != nil {
		log.Error("Failed to GetDiffShortStat: %v, repo: %v", err, ctx.Repo.Repository.FullName())
		return
	}

	ctx.Data["DiffShortStat"] = diffShortStat
}

func GetMergedBaseCommitID(ctx *context.Context, issue *issues_model.Issue) string {
	pull := issue.PullRequest

	var baseCommit string
	// Some migrated PR won't have any Base SHA and lose history, try to get one
	if pull.MergeBase == "" {
		var commitSHA, parentCommit string
		// If there is a head or a patch file, and it is readable, grab info
		commitSHA, err := ctx.Repo.GitRepo.GetRefCommitID(pull.GetGitHeadRefName())
		if err != nil {
			// Head File does not exist, try the patch
			commitSHA, err = ctx.Repo.GitRepo.ReadPatchCommit(pull.Index)
			if err == nil {
				// Recreate pull head in files for next time
				if err := gitrepo.UpdateRef(ctx, ctx.Repo.Repository, pull.GetGitHeadRefName(), commitSHA); err != nil {
					log.Error("Could not write head file", err)
				}
			} else {
				// There is no history available
				log.Trace("No history file available for PR %d", pull.Index)
			}
		}
		if commitSHA != "" {
			// Get immediate parent of the first commit in the patch, grab history back
			parentCommit, _, err = gitrepo.RunCmdString(ctx, ctx.Repo.Repository,
				gitcmd.NewCommand("rev-list", "-1", "--skip=1").AddDynamicArguments(commitSHA))
			if err == nil {
				parentCommit = strings.TrimSpace(parentCommit)
			}
			// Special case on Git < 2.25 that doesn't fail on immediate empty history
			if err != nil || parentCommit == "" {
				log.Info("No known parent commit for PR %d, error: %v", pull.Index, err)
				// bring at least partial history if it can work
				parentCommit = commitSHA
			}
		}
		baseCommit = parentCommit
	} else {
		// Keep an empty history or original commit
		baseCommit = pull.MergeBase
	}

	return baseCommit
}

// pullRequestViewInfo is a structured type for viewing pull request
// Refactoring plan:
// * move dynamic template-data-based variable into this struct
// * let backend handle complex logic, prepare everything, avoid plenty of "if" blocks in tmpl
type pullRequestViewInfo struct {
	issue *issues_model.Issue

	IsPullRequestBroken bool
	HeadBranchCommitID  string

	CompareInfo  git_service.CompareInfo
	MergeBoxInfo struct {
		// TODO: move "merge box" related template variables here in the future
	}

	StatusCheckData pullCommitStatusCheckData
	CommitStatuses  []*git_model.CommitStatus
}

func newPullRequestViewInfo() *pullRequestViewInfo {
	return &pullRequestViewInfo{}
}

func (prInfo *pullRequestViewInfo) prepareViewInfo(ctx *context.Context, issue *issues_model.Issue) {
	prInfo.issue = issue
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes

	if err := issue.PullRequest.LoadHeadRepo(ctx); err != nil {
		ctx.ServerError("LoadHeadRepo", err)
		return
	}

	if err := issue.PullRequest.LoadBaseRepo(ctx); err != nil {
		ctx.ServerError("LoadBaseRepo", err)
		return
	}

	// for the PR target branch selector
	ctx.Data["BaseBranch"] = issue.PullRequest.BaseBranch
	ctx.Data["HeadBranch"] = issue.PullRequest.HeadBranch
	ctx.Data["HeadUserName"] = issue.PullRequest.MustHeadUserName(ctx)

	if issue.PullRequest.HasMerged {
		prInfo.prepareViewMergedPullInfo(ctx)
	} else {
		prInfo.prepareViewOpenPullInfo(ctx)
	}
}

func (prInfo *pullRequestViewInfo) prepareViewFillInfo(ctx *context.Context, baseRef git.RefName) {
	prInfo.prepareViewFillCompareInfo(ctx, baseRef)
	if ctx.Written() {
		return
	}
	prInfo.prepareViewFillCommitStatusInfo(ctx)
}

func (prInfo *pullRequestViewInfo) prepareViewFillCompareInfo(ctx *context.Context, baseRef git.RefName) {
	var err error
	pull := prInfo.issue.PullRequest
	prInfo.CompareInfo, err = git_service.GetCompareInfo(ctx, ctx.Repo.Repository, ctx.Repo.Repository, ctx.Repo.GitRepo, baseRef, git.RefName(pull.GetGitHeadRefName()), false, false)
	if err != nil {
		isKnownErrorForBroken := gitcmd.IsStdErrorNotValidObjectName(err) ||
			// fatal: ambiguous argument 'origin': unknown revision or path not in the working tree.
			gitcmd.StderrContains(err, "unknown revision or path not in the working tree")
		if !isKnownErrorForBroken {
			log.Error("GetCompareInfo: %v", err)
		}
		prInfo.IsPullRequestBroken = true
	}

	prInfo.HeadBranchCommitID, err = getViewPullHeadBranchCommitID(ctx, pull)
	if err != nil {
		if !errors.Is(err, util.ErrNotExist) {
			log.Error("GetViewPullHeadBranchCommitID: %v", err)
		}
		prInfo.IsPullRequestBroken = true
	}
	if !pull.Issue.IsClosed && (prInfo.HeadBranchCommitID != prInfo.CompareInfo.HeadCommitID) {
		// if the PR is still open, but its "branch commit in head repo"
		// doesn't match "the PR's internal git ref commit in base repo", then the PR is broken
		prInfo.IsPullRequestBroken = true
	}

	ctx.Data["IsPullRequestBroken"] = prInfo.IsPullRequestBroken
	ctx.Data["NumCommits"] = len(prInfo.CompareInfo.Commits)
	ctx.Data["NumFiles"] = prInfo.CompareInfo.NumFiles
	prInfo.setTemplateDataMergeTarget(ctx)
}

func (prInfo *pullRequestViewInfo) prepareViewFillCommitStatusInfo(ctx *context.Context) {
	headCommitID := prInfo.CompareInfo.HeadCommitID
	if headCommitID == "" {
		return
	}

	repo := ctx.Repo.Repository
	statusCheckData := &prInfo.StatusCheckData

	commitStatuses, err := git_model.GetLatestCommitStatus(ctx, ctx.Repo.Repository.ID, prInfo.CompareInfo.HeadCommitID, db.ListOptionsAll)
	if err != nil {
		ctx.ServerError("GetLatestCommitStatus", err)
		return
	}
	if !ctx.Repo.CanRead(unit.TypeActions) {
		git_model.CommitStatusesHideActionsURL(ctx, commitStatuses)
	}

	prInfo.CommitStatuses = commitStatuses
	statusCheckData.ApproveLink = fmt.Sprintf("%s/actions/approve-all-checks?commit_id=%s", repo.Link(), headCommitID)
	statusCheckData.LatestCommitStatus = git_model.CalcCommitStatus(commitStatuses)
	ctx.Data["LatestCommitStatuses"] = commitStatuses
	ctx.Data["LatestCommitStatus"] = statusCheckData.LatestCommitStatus
	ctx.Data["StatusCheckData"] = &prInfo.StatusCheckData

	if !prInfo.issue.IsClosed {
		prInfo.prepareViewFillCommitStatusInfoForOpen(ctx)
	}
}

func (prInfo *pullRequestViewInfo) prepareViewFillCommitStatusInfoForOpen(ctx *context.Context) {
	issue := prInfo.issue
	statusCheckData := &prInfo.StatusCheckData
	commitStatuses := prInfo.CommitStatuses
	runs, err := actions_service.GetRunsFromCommitStatuses(ctx, commitStatuses)
	if err != nil {
		ctx.ServerError("GetRunsFromCommitStatuses", err)
		return
	}
	for _, run := range runs {
		if run.NeedApproval {
			statusCheckData.RequireApprovalRunCount++
		}
	}
	if statusCheckData.RequireApprovalRunCount > 0 {
		statusCheckData.CanApprove = ctx.Repo.CanWrite(unit.TypeActions)
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, ctx.Repo.Repository.ID, issue.PullRequest.BaseBranch)
	if err != nil {
		ctx.ServerError("LoadProtectedBranch", err)
		return
	}
	enableStatusCheck := pb != nil && pb.EnableStatusCheck
	ctx.Data["EnableStatusCheck"] = enableStatusCheck
	if !enableStatusCheck {
		return
	}
	var missingRequiredChecks []string
	for _, requiredContext := range pb.StatusCheckContexts {
		contextFound := false
		matchesRequiredContext := createRequiredContextMatcher(requiredContext)
		for _, presentStatus := range commitStatuses {
			if matchesRequiredContext(presentStatus.Context) {
				contextFound = true
				break
			}
		}

		if !contextFound {
			missingRequiredChecks = append(missingRequiredChecks, requiredContext)
		}
	}
	statusCheckData.MissingRequiredChecks = missingRequiredChecks

	statusCheckData.IsContextRequired = func(context string) bool {
		for _, c := range pb.StatusCheckContexts {
			if c == context {
				return true
			}
			if gp, err := glob.Compile(c); err != nil {
				// All newly created status_check_contexts are checked to ensure they are valid glob expressions before being stored in the database.
				// But some old status_check_context created before glob was introduced may be invalid glob expressions.
				// So log the error here for debugging.
				log.Error("compile glob %q: %v", c, err)
			} else if gp.Match(context) {
				return true
			}
		}
		return false
	}
	statusCheckData.RequiredChecksState = pull_service.MergeRequiredContextsCommitStatus(commitStatuses, pb.StatusCheckContexts)
}

// prepareViewMergedPullInfo show meta information for a merged pull request view page
func (prInfo *pullRequestViewInfo) prepareViewMergedPullInfo(ctx *context.Context) {
	ctx.Data["HasMerged"] = true
	baseCommit := GetMergedBaseCommitID(ctx, prInfo.issue)
	prInfo.prepareViewFillInfo(ctx, git.RefName(baseCommit))
}

type pullCommitStatusCheckData struct {
	MissingRequiredChecks   []string          // list of missing required checks
	IsContextRequired       func(string) bool // function to check whether a context is required
	RequireApprovalRunCount int               // number of workflow runs that require approval
	CanApprove              bool              // whether the user can approve workflow runs
	ApproveLink             string            // link to approve all checks
	RequiredChecksState     commitstatus.CommitStatusState
	LatestCommitStatus      *git_model.CommitStatus
}

func (d *pullCommitStatusCheckData) CommitStatusCheckPrompt(locale translation.Locale) string {
	if d.RequiredChecksState.IsPending() || len(d.MissingRequiredChecks) > 0 {
		return locale.TrString("repo.pulls.status_checking")
	} else if d.RequiredChecksState.IsSuccess() {
		if d.LatestCommitStatus != nil && d.LatestCommitStatus.State.IsFailure() {
			return locale.TrString("repo.pulls.status_checks_failure_optional")
		}
		return locale.TrString("repo.pulls.status_checks_success")
	} else if d.RequiredChecksState.IsWarning() {
		return locale.TrString("repo.pulls.status_checks_warning")
	} else if d.RequiredChecksState.IsFailure() {
		return locale.TrString("repo.pulls.status_checks_failure_required")
	} else if d.RequiredChecksState.IsError() {
		return locale.TrString("repo.pulls.status_checks_error")
	}
	return locale.TrString("repo.pulls.status_checking")
}

func getViewPullHeadBranchCommitID(ctx *context.Context, pull *issues_model.PullRequest) (string, error) {
	switch pull.Flow {
	case issues_model.PullRequestFlowGithub:
		if pull.HeadRepo == nil {
			return "", util.ErrNotExist
		}
		headGitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, pull.HeadRepo)
		if err != nil {
			return "", err
		}
		return headGitRepo.GetRefCommitID(git.RefNameFromBranch(pull.HeadBranch).String())
	case issues_model.PullRequestFlowAGit:
		baseGitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, pull.BaseRepo)
		if err != nil {
			return "", err
		}
		return baseGitRepo.GetRefCommitID(pull.GetGitHeadRefName())
	}
	setting.PanicInDevOrTesting("invalid pull request flow type: %v", pull.Flow)
	return "", util.ErrNotExist
}

func (prInfo *pullRequestViewInfo) prepareViewOpenPullInfo(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	if exist, _ := git_model.IsBranchExist(ctx, pull.BaseRepo.ID, pull.BaseBranch); !exist {
		// if base branch doesn't exist, prepare from the merge base
		ctx.Data["BaseBranchNotExist"] = true
		prInfo.prepareViewFillInfo(ctx, git.RefName(pull.MergeBase))
		return
	}

	prInfo.prepareViewFillInfo(ctx, git.RefNameFromBranch(pull.BaseBranch))
	if ctx.Written() {
		return
	}

	ctx.Data["PullHeadCommitID"] = prInfo.CompareInfo.HeadCommitID

	if prInfo.CompareInfo.HeadCommitID == prInfo.CompareInfo.MergeBase {
		ctx.Data["IsNothingToCompare"] = true
	}

	// this one is used by both sidebar and merge-box
	if pull.IsWorkInProgress(ctx) {
		ctx.Data["IsPullWorkInProgress"] = true
		ctx.Data["WorkInProgressPrefix"] = pull.GetWorkInProgressPrefix(ctx)
	}
}

func createRequiredContextMatcher(requiredContext string) func(string) bool {
	if gp, err := glob.Compile(requiredContext); err == nil {
		return func(contextToCheck string) bool {
			return gp.Match(contextToCheck)
		}
	}

	return func(contextToCheck string) bool {
		return requiredContext == contextToCheck
	}
}

type pullCommitList struct {
	Commits             []pull_service.CommitInfo `json:"commits"`
	LastReviewCommitSha string                    `json:"last_review_commit_sha"`
	Locale              map[string]any            `json:"locale"`
}

// GetPullCommits get all commits for given pull request
func GetPullCommits(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	resp := &pullCommitList{}

	commits, lastReviewCommitSha, err := pull_service.GetPullCommits(ctx, ctx.Repo.GitRepo, ctx.Doer, issue)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	// Get the needed locale
	resp.Locale = map[string]any{
		"lang":                                ctx.Locale.Language(),
		"show_all_commits":                    ctx.Tr("repo.pulls.show_all_commits"),
		"stats_num_commits":                   ctx.TrN(len(commits), "repo.activity.git_stats_commit_1", "repo.activity.git_stats_commit_n", len(commits)),
		"show_changes_since_your_last_review": ctx.Tr("repo.pulls.show_changes_since_your_last_review"),
		"select_commit_hold_shift_for_range":  ctx.Tr("repo.pulls.select_commit_hold_shift_for_range"),
	}

	resp.Commits = commits
	resp.LastReviewCommitSha = lastReviewCommitSha

	ctx.JSON(http.StatusOK, resp)
}

// ViewPullCommits show commits for a pull request
func ViewPullCommits(ctx *context.Context) {
	ctx.Data["PageIsPullList"] = true
	ctx.Data["PageIsPullCommits"] = true

	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	prViewInfo := newPullRequestViewInfo()
	prViewInfo.prepareViewInfo(ctx, issue)
	if ctx.Written() {
		return
	}
	prCompareInfo := &prViewInfo.CompareInfo
	if prCompareInfo.HeadCommitID == "" {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name

	commits, err := processGitCommits(ctx, prCompareInfo.Commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}
	ctx.Data["Commits"] = commits
	ctx.Data["CommitCount"] = len(commits)

	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)
	ctx.Data["IsIssuePoster"] = ctx.IsSigned && issue.IsPoster(ctx.Doer.ID)

	// For PR commits page
	PrepareBranchList(ctx)
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, tplPullCommits)
}

func indexCommit(commits []*git.Commit, commitID string) *git.Commit {
	for i := range commits {
		if commits[i].ID.String() == commitID {
			return commits[i]
		}
	}
	return nil
}

// ViewPullFiles render pull request changed files list page
func viewPullFiles(ctx *context.Context, beforeCommitID, afterCommitID string) {
	ctx.Data["PageIsPullList"] = true
	ctx.Data["PageIsPullFiles"] = true

	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest

	gitRepo := ctx.Repo.GitRepo

	prViewInfo := newPullRequestViewInfo()
	prViewInfo.prepareViewInfo(ctx, issue)
	if ctx.Written() {
		return
	}
	prCompareInfo := &prViewInfo.CompareInfo
	if prCompareInfo.HeadCommitID == "" {
		ctx.NotFound(nil)
		return
	}

	headCommitID := prCompareInfo.HeadCommitID
	isSingleCommit := beforeCommitID == "" && afterCommitID != ""
	ctx.Data["IsShowingOnlySingleCommit"] = isSingleCommit
	isShowAllCommits := (beforeCommitID == "" || beforeCommitID == prCompareInfo.MergeBase) && (afterCommitID == "" || afterCommitID == headCommitID)
	ctx.Data["IsShowingAllCommits"] = isShowAllCommits

	if afterCommitID == "" || afterCommitID == headCommitID {
		afterCommitID = headCommitID
	}
	afterCommit := indexCommit(prCompareInfo.Commits, afterCommitID)
	if afterCommit == nil {
		ctx.HTTPError(http.StatusBadRequest, "after commit not found in PR commits")
		return
	}

	var beforeCommit *git.Commit
	var err error
	if !isSingleCommit {
		if beforeCommitID == "" || beforeCommitID == prCompareInfo.MergeBase {
			beforeCommitID = prCompareInfo.MergeBase
			// merge base commit is not in the list of the pull request commits
			beforeCommit, err = gitRepo.GetCommit(beforeCommitID)
			if err != nil {
				ctx.ServerError("GetCommit", err)
				return
			}
		} else {
			beforeCommit = indexCommit(prCompareInfo.Commits, beforeCommitID)
			if beforeCommit == nil {
				ctx.HTTPError(http.StatusBadRequest, "before commit not found in PR commits")
				return
			}
		}
	} else {
		beforeCommit, err = afterCommit.Parent(0)
		if err != nil {
			ctx.ServerError("Parent", err)
			return
		}
		beforeCommitID = beforeCommit.ID.String()
	}

	ctx.Data["MergeBase"] = prCompareInfo.MergeBase
	ctx.Data["AfterCommitID"] = afterCommitID
	ctx.Data["BeforeCommitID"] = beforeCommitID

	maxLines, maxFiles := setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles
	files := ctx.FormStrings("files")
	fileOnly := ctx.FormBool("file-only")
	if fileOnly && (len(files) == 2 || len(files) == 1) {
		maxLines, maxFiles = -1, -1
	}

	diffOptions := &gitdiff.DiffOptions{
		BeforeCommitID:     beforeCommitID,
		AfterCommitID:      afterCommitID,
		SkipTo:             ctx.FormString("skip-to"),
		MaxLines:           maxLines,
		MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:           maxFiles,
		WhitespaceBehavior: gitdiff.GetWhitespaceFlag(ctx.Data["WhitespaceBehavior"].(string)),
	}

	diff, err := gitdiff.GetDiffForRender(ctx, ctx.Repo.RepoLink, gitRepo, diffOptions, files...)
	if err != nil {
		ctx.ServerError("GetDiff", err)
		return
	}

	// if we're not logged in or only a single commit (or commit range) is shown we
	// have to load only the diff and not get the viewed information
	// as the viewed information is designed to be loaded only on latest PR
	// diff and if you're signed in.
	var reviewState *pull_model.ReviewState
	var numViewedFiles int
	if ctx.IsSigned && isShowAllCommits {
		reviewState, err = gitdiff.SyncUserSpecificDiff(ctx, ctx.Doer.ID, pull, gitRepo, diff, diffOptions)
		if err != nil {
			ctx.ServerError("SyncUserSpecificDiff", err)
			return
		}
		if reviewState != nil {
			numViewedFiles = reviewState.GetViewedFileCount()
		}
	}

	diffShortStat, err := gitdiff.GetDiffShortStat(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, beforeCommitID, afterCommitID)
	if err != nil {
		ctx.ServerError("GetDiffShortStat", err)
		return
	}
	ctx.Data["DiffShortStat"] = diffShortStat
	ctx.Data["NumViewedFiles"] = numViewedFiles

	ctx.PageData["prReview"] = map[string]any{
		"numberOfFiles":       diffShortStat.NumFiles,
		"numberOfViewedFiles": numViewedFiles,
	}

	if err = diff.LoadComments(ctx, issue, ctx.Doer, ctx.Data["ShowOutdatedComments"].(bool)); err != nil {
		ctx.ServerError("LoadComments", err)
		return
	}

	allComments := issues_model.CommentList{}
	for _, file := range diff.Files {
		for _, section := range file.Sections {
			for _, line := range section.Lines {
				allComments = append(allComments, line.Comments...)
			}
		}
	}
	if err := allComments.LoadAttachments(ctx); err != nil {
		ctx.ServerError("LoadAttachments", err)
		return
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pull.BaseRepoID, pull.BaseBranch)
	if err != nil {
		ctx.ServerError("LoadProtectedBranch", err)
		return
	}

	if pb != nil {
		protectedFilePatterns := pb.GetProtectedFilePatterns()
		if len(protectedFilePatterns) != 0 {
			for _, file := range diff.Files {
				file.IsProtected = pb.IsProtectedFile(protectedFilePatterns, file.Name)
			}
		}
	}

	if !fileOnly {
		// note: use mergeBase is set to false because we already have the merge base from the pull request info
		diffTree, err := gitdiff.GetDiffTree(ctx, gitRepo, false, beforeCommitID, afterCommitID)
		if err != nil {
			ctx.ServerError("GetDiffTree", err)
			return
		}
		var filesViewedState map[string]pull_model.ViewedState
		if reviewState != nil {
			filesViewedState = reviewState.UpdatedFiles
		}

		renderedIconPool := fileicon.NewRenderedIconPool()
		ctx.PageData["DiffFileTree"] = transformDiffTreeForWeb(renderedIconPool, diffTree, filesViewedState)
		ctx.PageData["FolderIcon"] = fileicon.RenderEntryIconHTML(renderedIconPool, fileicon.EntryInfoFolder())
		ctx.PageData["FolderOpenIcon"] = fileicon.RenderEntryIconHTML(renderedIconPool, fileicon.EntryInfoFolderOpen())
		ctx.Data["FileIconPoolHTML"] = renderedIconPool.RenderToHTML()
	}

	ctx.Data["Diff"] = diff
	ctx.Data["DiffBlobExcerptData"] = &gitdiff.DiffBlobExcerptData{
		BaseLink:       ctx.Repo.RepoLink + "/blob_excerpt",
		PullIssueIndex: pull.Index,
		DiffStyle:      GetDiffViewStyle(ctx),
		AfterCommitID:  afterCommitID,
	}
	ctx.Data["DiffNotAvailable"] = diffShortStat.NumFiles == 0

	if ctx.Data["CanMarkConversation"], err = issues_model.CanMarkConversation(ctx, issue, ctx.Doer); err != nil {
		ctx.ServerError("CanMarkConversation", err)
		return
	}

	setCompareContext(ctx, beforeCommit, afterCommit, ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)

	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	ctx.Data["Assignees"] = shared_user.MakeSelfOnTop(ctx.Doer, assigneeUsers)

	currentReview, err := issues_model.GetCurrentReview(ctx, ctx.Doer, issue)
	if err != nil && !issues_model.IsErrReviewNotExist(err) {
		ctx.ServerError("GetCurrentReview", err)
		return
	}
	numPendingCodeComments := int64(0)
	if currentReview != nil {
		numPendingCodeComments, err = issues_model.CountComments(ctx, &issues_model.FindCommentsOptions{
			Type:     issues_model.CommentTypeCode,
			ReviewID: currentReview.ID,
			IssueID:  issue.ID,
		})
		if err != nil {
			ctx.ServerError("CountComments", err)
			return
		}
	}
	ctx.Data["CurrentReview"] = currentReview
	ctx.Data["PendingCodeCommentNumber"] = numPendingCodeComments

	ctx.Data["IsIssuePoster"] = ctx.Doer != nil && issue.IsPoster(ctx.Doer.ID)
	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)

	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	// For files changed page
	PrepareBranchList(ctx)
	if ctx.Written() {
		return
	}
	upload.AddUploadContext(ctx, "comment")

	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}
	if isShowAllCommits && pull.Flow == issues_model.PullRequestFlowGithub {
		if err := pull.LoadHeadRepo(ctx); err != nil {
			ctx.ServerError("LoadHeadRepo", err)
			return
		}

		if pull.HeadRepo != nil {
			if !pull.HasMerged && ctx.Doer != nil {
				perm, err := access_model.GetDoerRepoPermission(ctx, pull.HeadRepo, ctx.Doer)
				if err != nil {
					ctx.ServerError("GetDoerRepoPermission", err)
					return
				}

				if perm.CanWrite(unit.TypeCode) || issues_model.CanMaintainerWriteToBranch(ctx, perm, pull.HeadBranch, ctx.Doer) {
					ctx.Data["CanEditFile"] = true
					ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.edit_this_file")
					ctx.Data["HeadRepoLink"] = pull.HeadRepo.Link()
					ctx.Data["HeadBranchName"] = pull.HeadBranch
					ctx.Data["BackToLink"] = setting.AppSubURL + ctx.Req.URL.RequestURI()
				}
			}
		}
	}

	ctx.HTML(http.StatusOK, tplPullFiles)
}

func ViewPullFilesForSingleCommit(ctx *context.Context) {
	// it doesn't support showing files from mergebase to the special commit
	// otherwise it will be ambiguous
	viewPullFiles(ctx, "", ctx.PathParam("sha"))
}

func ViewPullFilesForRange(ctx *context.Context) {
	viewPullFiles(ctx, ctx.PathParam("shaFrom"), ctx.PathParam("shaTo"))
}

func ViewPullFilesForAllCommitsOfPr(ctx *context.Context) {
	viewPullFiles(ctx, "", "")
}

// UpdatePullRequest merge PR's baseBranch into headBranch
func UpdatePullRequest(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	if issue.IsClosed {
		ctx.NotFound(nil)
		return
	}
	if issue.PullRequest.HasMerged {
		ctx.NotFound(nil)
		return
	}

	rebase := ctx.FormString("style") == "rebase"

	if err := issue.PullRequest.LoadBaseRepo(ctx); err != nil {
		ctx.ServerError("LoadBaseRepo", err)
		return
	}
	if err := issue.PullRequest.LoadHeadRepo(ctx); err != nil {
		ctx.ServerError("LoadHeadRepo", err)
		return
	}

	allowedUpdateByMerge, allowedUpdateByRebase, err := pull_service.IsUserAllowedToUpdate(ctx, issue.PullRequest, ctx.Doer)
	if err != nil {
		ctx.ServerError("IsUserAllowedToMerge", err)
		return
	}

	// ToDo: add check if maintainers are allowed to change branch ... (need migration & co)
	if (!allowedUpdateByMerge && !rebase) || (rebase && !allowedUpdateByRebase) {
		ctx.Flash.Error(ctx.Tr("repo.pulls.update_not_allowed"))
		ctx.Redirect(issue.Link())
		return
	}

	// default merge commit message
	message := fmt.Sprintf("Merge branch '%s' into %s", issue.PullRequest.BaseBranch, issue.PullRequest.HeadBranch)

	// The update process should not be cancelled by the user
	// so we set the context to be a background context
	if err = pull_service.Update(graceful.GetManager().ShutdownContext(), issue.PullRequest, ctx.Doer, message, rebase); err != nil {
		if pull_service.IsErrMergeConflicts(err) {
			conflictError := err.(pull_service.ErrMergeConflicts)
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.pulls.merge_conflict"),
				"Summary": ctx.Tr("repo.pulls.merge_conflict_summary"),
				"Details": utils.EscapeFlashErrorString(conflictError.StdErr) + "\n" + utils.EscapeFlashErrorString(conflictError.StdOut),
			})
			if err != nil {
				ctx.ServerError("UpdatePullRequest.HTMLString", err)
				return
			}
			ctx.Flash.Error(flashError)
			ctx.Redirect(issue.Link())
			return
		} else if pull_service.IsErrRebaseConflicts(err) {
			conflictError := err.(pull_service.ErrRebaseConflicts)
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.pulls.rebase_conflict", utils.EscapeFlashErrorString(conflictError.CommitSHA)),
				"Summary": ctx.Tr("repo.pulls.rebase_conflict_summary"),
				"Details": utils.EscapeFlashErrorString(conflictError.StdErr) + "\n" + utils.EscapeFlashErrorString(conflictError.StdOut),
			})
			if err != nil {
				ctx.ServerError("UpdatePullRequest.HTMLString", err)
				return
			}
			ctx.Flash.Error(flashError)
			ctx.Redirect(issue.Link())
			return
		}
		ctx.Flash.Error(err.Error())
		ctx.Redirect(issue.Link())
		return
	}

	time.Sleep(1 * time.Second)

	ctx.Flash.Success(ctx.Tr("repo.pulls.update_branch_success"))
	ctx.Redirect(issue.Link())
}

// MergePullRequest response for merging pull request
func MergePullRequest(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.MergePullRequestForm)
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}

	pr := issue.PullRequest
	pr.Issue = issue
	pr.Issue.Repo = ctx.Repo.Repository

	manuallyMerged := repo_model.MergeStyle(form.Do) == repo_model.MergeStyleManuallyMerged

	mergeCheckType := pull_service.MergeCheckTypeGeneral
	if form.MergeWhenChecksSucceed {
		mergeCheckType = pull_service.MergeCheckTypeAuto
	}
	if manuallyMerged {
		mergeCheckType = pull_service.MergeCheckTypeManually
	}

	// start with merging by checking
	if err := pull_service.CheckPullMergeable(ctx, ctx.Doer, &ctx.Repo.Permission, pr, mergeCheckType, repo_model.MergeStyle(form.Do), form.ForceMerge); err != nil {
		switch {
		case errors.Is(err, pull_service.ErrIsClosed):
			if issue.IsPull {
				ctx.JSONError(ctx.Tr("repo.pulls.is_closed"))
			} else {
				ctx.JSONError(ctx.Tr("repo.issues.closed_title"))
			}
		case errors.Is(err, pull_service.ErrNoPermissionToMerge):
			ctx.JSONError(ctx.Tr("repo.pulls.update_not_allowed"))
		case errors.Is(err, pull_service.ErrHasMerged):
			ctx.JSONError(ctx.Tr("repo.pulls.has_merged"))
		case errors.Is(err, pull_service.ErrIsWorkInProgress):
			ctx.JSONError(ctx.Tr("repo.pulls.no_merge_wip"))
		case errors.Is(err, pull_service.ErrNotMergeableState):
			ctx.JSONError(ctx.Tr("repo.pulls.no_merge_not_ready"))
		case errors.Is(err, pull_service.ErrNotReadyToMerge):
			ctx.JSONError(ctx.Tr("repo.pulls.no_merge_not_ready"))
		case asymkey_service.IsErrWontSign(err):
			ctx.JSONError(err.Error()) // has no translation ...
		case errors.Is(err, pull_service.ErrHeadCommitsNotAllVerified):
			ctx.JSONError(ctx.Tr("repo.pulls.require_signed_head_commits_unverified"))
		case errors.Is(err, pull_service.ErrDependenciesLeft):
			ctx.JSONError(ctx.Tr("repo.issues.dependency.pr_close_blocked"))
		default:
			ctx.ServerError("WebCheck", err)
		}

		return
	}

	// handle manually-merged mark
	if manuallyMerged {
		if err := pull_service.MergedManually(ctx, pr, ctx.Doer, ctx.Repo.GitRepo, form.MergeCommitID); err != nil {
			switch {
			case pull_service.IsErrInvalidMergeStyle(err):
				ctx.JSONError(ctx.Tr("repo.pulls.invalid_merge_option"))
			case strings.Contains(err.Error(), "Wrong commit ID"):
				ctx.JSONError(ctx.Tr("repo.pulls.wrong_commit_id"))
			default:
				ctx.ServerError("MergedManually", err)
			}

			return
		}

		ctx.JSONRedirect(issue.Link())
		return
	}

	message := strings.TrimSpace(form.MergeTitleField)
	if len(message) == 0 {
		var err error
		message, _, err = pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pr, repo_model.MergeStyle(form.Do))
		if err != nil {
			ctx.ServerError("GetDefaultMergeMessage", err)
			return
		}
	}

	form.MergeMessageField = strings.TrimSpace(form.MergeMessageField)
	if len(form.MergeMessageField) > 0 {
		message += "\n\n" + form.MergeMessageField
	}

	// There is always a checkbox on the UI (the DeleteBranchAfterMerge is nil if the checkbox is not checked),
	// just use the user's choice, don't use pull_service.ShouldDeleteBranchAfterMerge to decide
	deleteBranchAfterMerge := optional.FromPtr(form.DeleteBranchAfterMerge).Value()

	if form.MergeWhenChecksSucceed {
		// delete all scheduled auto merges
		_ = pull_model.DeleteScheduledAutoMerge(ctx, pr.ID)
		// schedule auto merge
		scheduled, err := automerge.ScheduleAutoMerge(ctx, ctx.Doer, pr, repo_model.MergeStyle(form.Do), message, deleteBranchAfterMerge)
		if err != nil {
			ctx.ServerError("ScheduleAutoMerge", err)
			return
		} else if scheduled {
			// nothing more to do ...
			ctx.Flash.Success(ctx.Tr("repo.pulls.auto_merge_newly_scheduled"))
			ctx.JSONRedirect(fmt.Sprintf("%s/pulls/%d", ctx.Repo.RepoLink, pr.Index))
			return
		}
	}

	if err := pull_service.Merge(ctx, pr, ctx.Doer, repo_model.MergeStyle(form.Do), form.HeadCommitID, message, false); err != nil {
		if pull_service.IsErrInvalidMergeStyle(err) {
			ctx.JSONError(ctx.Tr("repo.pulls.invalid_merge_option"))
		} else if pull_service.IsErrMergeConflicts(err) {
			conflictError := err.(pull_service.ErrMergeConflicts)
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.editor.merge_conflict"),
				"Summary": ctx.Tr("repo.editor.merge_conflict_summary"),
				"Details": utils.EscapeFlashErrorString(conflictError.StdErr) + "\n" + utils.EscapeFlashErrorString(conflictError.StdOut),
			})
			if err != nil {
				ctx.ServerError("MergePullRequest.HTMLString", err)
				return
			}
			ctx.Flash.Error(flashError)
			ctx.JSONRedirect(issue.Link())
		} else if pull_service.IsErrRebaseConflicts(err) {
			conflictError := err.(pull_service.ErrRebaseConflicts)
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.pulls.rebase_conflict", utils.EscapeFlashErrorString(conflictError.CommitSHA)),
				"Summary": ctx.Tr("repo.pulls.rebase_conflict_summary"),
				"Details": utils.EscapeFlashErrorString(conflictError.StdErr) + "\n" + utils.EscapeFlashErrorString(conflictError.StdOut),
			})
			if err != nil {
				ctx.ServerError("MergePullRequest.HTMLString", err)
				return
			}
			ctx.Flash.Error(flashError)
			ctx.JSONRedirect(issue.Link())
		} else if pull_service.IsErrMergeUnrelatedHistories(err) {
			log.Debug("MergeUnrelatedHistories error: %v", err)
			ctx.Flash.Error(ctx.Tr("repo.pulls.unrelated_histories"))
			ctx.JSONRedirect(issue.Link())
		} else if git.IsErrPushOutOfDate(err) {
			log.Debug("MergePushOutOfDate error: %v", err)
			ctx.Flash.Error(ctx.Tr("repo.pulls.merge_out_of_date"))
			ctx.JSONRedirect(issue.Link())
		} else if pull_service.IsErrSHADoesNotMatch(err) {
			log.Debug("MergeHeadOutOfDate error: %v", err)
			ctx.Flash.Error(ctx.Tr("repo.pulls.head_out_of_date"))
			ctx.JSONRedirect(issue.Link())
		} else if git.IsErrPushRejected(err) {
			log.Debug("MergePushRejected error: %v", err)
			pushrejErr := err.(*git.ErrPushRejected)
			message := pushrejErr.Message
			if len(message) == 0 {
				ctx.Flash.Error(ctx.Tr("repo.pulls.push_rejected_no_message"))
			} else {
				flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
					"Message": ctx.Tr("repo.pulls.push_rejected"),
					"Summary": ctx.Tr("repo.pulls.push_rejected_summary"),
					"Details": utils.EscapeFlashErrorString(pushrejErr.Message),
				})
				if err != nil {
					ctx.ServerError("MergePullRequest.HTMLString", err)
					return
				}
				ctx.Flash.Error(flashError)
			}
			ctx.JSONRedirect(issue.Link())
		} else {
			ctx.ServerError("Merge", err)
		}
		return
	}
	log.Trace("Pull request merged: %d", pr.ID)

	if err := stopTimerIfAvailable(ctx, ctx.Doer, issue); err != nil {
		ctx.ServerError("stopTimerIfAvailable", err)
		return
	}

	log.Trace("Pull request merged: %d", pr.ID)

	if deleteBranchAfterMerge {
		deleteBranchAfterMergeAndFlashMessage(ctx, pr.ID)
		if ctx.Written() {
			return
		}
	}
	ctx.JSONRedirect(issue.Link())
}

func deleteBranchAfterMergeAndFlashMessage(ctx *context.Context, prID int64) {
	var fullBranchName string
	err := repo_service.DeleteBranchAfterMerge(ctx, ctx.Doer, prID, &fullBranchName)
	if errors.Is(err, util.ErrPermissionDenied) || errors.Is(err, util.ErrNotExist) {
		// no need to show error to end users if no permission or branch not exist
		log.Debug("DeleteBranchAfterMerge (ignore unnecessary error): %v", err)
		return
	} else if errTr := util.ErrorAsTranslatable(err); errTr != nil {
		ctx.Flash.Error(errTr.Translate(ctx.Locale))
		return
	} else if err == nil {
		ctx.Flash.Success(ctx.Tr("repo.branch.deletion_success", fullBranchName))
		return
	}
	// catch unknown errors
	ctx.ServerError("DeleteBranchAfterMerge", err)
}

// CancelAutoMergePullRequest cancels a scheduled pr
func CancelAutoMergePullRequest(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}

	exist, autoMerge, err := pull_model.GetScheduledMergeByPullID(ctx, issue.PullRequest.ID)
	if err != nil {
		ctx.ServerError("GetScheduledMergeByPullID", err)
		return
	}
	if !exist {
		ctx.NotFound(nil)
		return
	}

	if ctx.Doer.ID != autoMerge.DoerID {
		allowed, err := pull_service.IsUserAllowedToMerge(ctx, issue.PullRequest, ctx.Repo.Permission, ctx.Doer)
		if err != nil {
			ctx.ServerError("IsUserAllowedToMerge", err)
			return
		}
		if !allowed {
			ctx.HTTPError(http.StatusForbidden, "user has no permission to cancel the scheduled auto merge")
			return
		}
	}

	if err := automerge.RemoveScheduledAutoMerge(ctx, ctx.Doer, issue.PullRequest); err != nil {
		if db.IsErrNotExist(err) {
			ctx.Flash.Error(ctx.Tr("repo.pulls.auto_merge_not_scheduled"))
			ctx.Redirect(fmt.Sprintf("%s/pulls/%d", ctx.Repo.RepoLink, issue.Index))
			return
		}
		ctx.ServerError("RemoveScheduledAutoMerge", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.pulls.auto_merge_canceled_schedule"))
	ctx.Redirect(fmt.Sprintf("%s/pulls/%d", ctx.Repo.RepoLink, issue.Index))
}

func stopTimerIfAvailable(ctx *context.Context, user *user_model.User, issue *issues_model.Issue) error {
	_, err := issues_model.FinishIssueStopwatch(ctx, user, issue)
	return err
}

func PullsNewRedirect(ctx *context.Context) {
	branch := ctx.PathParam("*")
	redirectRepo := ctx.Repo.Repository
	repo := ctx.Repo.Repository
	if repo.IsFork {
		if err := repo.GetBaseRepo(ctx); err != nil {
			ctx.ServerError("GetBaseRepo", err)
			return
		}
		redirectRepo = repo.BaseRepo
		branch = fmt.Sprintf("%s:%s", repo.OwnerName, branch)
	}
	ctx.Redirect(fmt.Sprintf("%s/compare/%s...%s?expand=1", redirectRepo.Link(), util.PathEscapeSegments(redirectRepo.DefaultBranch), util.PathEscapeSegments(branch)))
}

// CompareAndPullRequestPost response for creating pull request
func CompareAndPullRequestPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateIssueForm)
	ctx.Data["Title"] = ctx.Tr("repo.pulls.compare_changes")
	ctx.Data["PageIsComparePull"] = true
	ctx.Data["IsDiffCompare"] = true
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWrite(unit.TypePullRequests)

	var (
		repo        = ctx.Repo.Repository
		attachments []string
	)

	ci := ParseCompareInfo(ctx)
	if ctx.Written() {
		return
	}

	validateRet := ValidateRepoMetasForNewIssue(ctx, *form, true)
	if ctx.Written() {
		return
	}

	labelIDs, assigneeIDs, milestoneID, projectID := validateRet.LabelIDs, validateRet.AssigneeIDs, validateRet.MilestoneID, validateRet.ProjectID

	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	if util.IsEmptyString(form.Title) {
		ctx.JSONError(ctx.Tr("repo.issues.new.title_empty"))
		return
	}

	// Check if a pull request already exists with the same head and base branch.
	pr, err := issues_model.GetUnmergedPullRequest(ctx, ci.HeadRepo.ID, repo.ID, ci.HeadRef.ShortName(), ci.BaseRef.ShortName(), issues_model.PullRequestFlowGithub)
	if err != nil && !issues_model.IsErrPullRequestNotExist(err) {
		ctx.ServerError("GetUnmergedPullRequest", err)
		return
	}
	if pr != nil {
		ctx.JSONError(ctx.Tr("repo.pulls.new.already_existed"))
		return
	}

	content := form.Content
	if filename := ctx.Req.Form.Get("template-file"); filename != "" {
		if template, err := issue_template.UnmarshalFromRepo(ctx.Repo.GitRepo, ctx.Repo.Repository.DefaultBranch, filename); err == nil {
			content = issue_template.RenderToMarkdown(template, ctx.Req.Form)
		}
	}

	pullIssue := &issues_model.Issue{
		RepoID:      repo.ID,
		Repo:        repo,
		Title:       form.Title,
		PosterID:    ctx.Doer.ID,
		Poster:      ctx.Doer,
		MilestoneID: milestoneID,
		IsPull:      true,
		Content:     content,
	}
	pullRequest := &issues_model.PullRequest{
		HeadRepoID:          ci.HeadRepo.ID,
		BaseRepoID:          repo.ID,
		HeadBranch:          ci.HeadRef.ShortName(),
		BaseBranch:          ci.BaseRef.ShortName(),
		HeadRepo:            ci.HeadRepo,
		BaseRepo:            repo,
		MergeBase:           ci.MergeBase,
		Type:                issues_model.PullRequestGitea,
		AllowMaintainerEdit: form.AllowMaintainerEdit,
	}
	// FIXME: check error in the case two people send pull request at almost same time, give nice error prompt
	// instead of 500.
	prOpts := &pull_service.NewPullRequestOptions{
		Repo:            repo,
		Issue:           pullIssue,
		LabelIDs:        labelIDs,
		AttachmentUUIDs: attachments,
		PullRequest:     pullRequest,
		AssigneeIDs:     assigneeIDs,
		Reviewers:       validateRet.Reviewers,
		TeamReviewers:   validateRet.TeamReviewers,
		ProjectID:       projectID,
	}
	if err := pull_service.NewPullRequest(ctx, prOpts); err != nil {
		switch {
		case repo_model.IsErrUserDoesNotHaveAccessToRepo(err):
			ctx.HTTPError(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err.Error())
		case git.IsErrPushRejected(err):
			pushrejErr := err.(*git.ErrPushRejected)
			message := pushrejErr.Message
			if len(message) == 0 {
				ctx.JSONError(ctx.Tr("repo.pulls.push_rejected_no_message"))
				return
			}
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.pulls.push_rejected"),
				"Summary": ctx.Tr("repo.pulls.push_rejected_summary"),
				"Details": utils.EscapeFlashErrorString(pushrejErr.Message),
			})
			if err != nil {
				ctx.ServerError("CompareAndPullRequest.HTMLString", err)
				return
			}
			ctx.JSONError(flashError)
		case errors.Is(err, user_model.ErrBlockedUser):
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.pulls.push_rejected"),
				"Summary": ctx.Tr("repo.pulls.new.blocked_user"),
			})
			if err != nil {
				ctx.ServerError("CompareAndPullRequest.HTMLString", err)
				return
			}
			ctx.JSONError(flashError)
		case errors.Is(err, issues_model.ErrMustCollaborator):
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.pulls.push_rejected"),
				"Summary": ctx.Tr("repo.pulls.new.must_collaborator"),
			})
			if err != nil {
				ctx.ServerError("CompareAndPullRequest.HTMLString", err)
				return
			}
			ctx.JSONError(flashError)
		default:
			// It's an unexpected error.
			// If it happens, we should add another case to handle it.
			log.Error("Unexpected error of NewPullRequest: %T %s", err, err)
			ctx.ServerError("CompareAndPullRequest", err)
		}
		return
	}

	log.Trace("Pull request created: %d/%d", repo.ID, pullIssue.ID)
	ctx.JSONRedirect(pullIssue.Link())
}

// CleanUpPullRequest responses for delete merged branch when PR has been merged
// Used by "DeleteBranchLink" for "delete branch" button
func CleanUpPullRequest(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	deleteBranchAfterMergeAndFlashMessage(ctx, issue.PullRequest.ID)
	if ctx.Written() {
		return
	}
	ctx.JSONRedirect(issue.Link())
}

// DownloadPullDiff render a pull's raw diff
func DownloadPullDiff(ctx *context.Context) {
	DownloadPullDiffOrPatch(ctx, false)
}

// DownloadPullPatch render a pull's raw patch
func DownloadPullPatch(ctx *context.Context) {
	DownloadPullDiffOrPatch(ctx, true)
}

// DownloadPullDiffOrPatch render a pull's raw diff or patch
func DownloadPullDiffOrPatch(ctx *context.Context, patch bool) {
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetPullRequestByIndex", err)
		}
		return
	}

	binary := ctx.FormBool("binary")

	if err := pull_service.DownloadDiffOrPatch(ctx, pr, ctx, patch, binary); err != nil {
		ctx.ServerError("DownloadDiffOrPatch", err)
		return
	}
}

// UpdatePullRequestTarget change pull request's target branch
func UpdatePullRequestTarget(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}
	pr := issue.PullRequest
	if !issue.IsPull {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	targetBranch := ctx.FormTrim("target_branch")
	if len(targetBranch) == 0 {
		ctx.HTTPError(http.StatusNoContent)
		return
	}

	if err := pull_service.ChangeTargetBranch(ctx, pr, ctx.Doer, targetBranch); err != nil {
		switch {
		case git_model.IsErrBranchNotExist(err):
			errorMessage := ctx.Tr("form.target_branch_not_exist")

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusBadRequest, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		case issues_model.IsErrPullRequestAlreadyExists(err):
			err := err.(issues_model.ErrPullRequestAlreadyExists)

			RepoRelPath := ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name
			errorMessage := ctx.Tr("repo.pulls.has_pull_request", html.EscapeString(ctx.Repo.RepoLink+"/pulls/"+strconv.FormatInt(err.IssueID, 10)), html.EscapeString(RepoRelPath), err.IssueID) // FIXME: Creates url inside locale string

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusConflict, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		case issues_model.IsErrIssueIsClosed(err):
			errorMessage := ctx.Tr("repo.pulls.is_closed")

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusConflict, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		case pull_service.IsErrPullRequestHasMerged(err):
			errorMessage := ctx.Tr("repo.pulls.has_merged")

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusConflict, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		case git_model.IsErrBranchesEqual(err):
			errorMessage := ctx.Tr("repo.pulls.nothing_to_compare")

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusBadRequest, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		default:
			ctx.ServerError("UpdatePullRequestTarget", err)
		}
		return
	}
	notify_service.PullRequestChangeTargetBranch(ctx, ctx.Doer, pr, targetBranch)

	ctx.JSON(http.StatusOK, map[string]any{
		"base_branch": pr.BaseBranch,
	})
}

// SetAllowEdits allow edits from maintainers to PRs
func SetAllowEdits(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.UpdateAllowEditsForm)

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetPullRequestByIndex", err)
		}
		return
	}

	if err := pull_service.SetAllowEdits(ctx, ctx.Doer, pr, form.AllowMaintainerEdit); err != nil {
		if errors.Is(err, pull_service.ErrUserHasNoPermissionForAction) {
			ctx.HTTPError(http.StatusForbidden)
			return
		}
		ctx.ServerError("SetAllowEdits", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"allow_maintainer_edit": pr.AllowMaintainerEdit,
	})
}
