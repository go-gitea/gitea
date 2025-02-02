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
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	issue_template "code.gitea.io/gitea/modules/issue/template"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/automerge"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/gitdiff"
	notify_service "code.gitea.io/gitea/services/notify"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
	user_service "code.gitea.io/gitea/services/user"

	"github.com/gobwas/glob"
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
			ctx.NotFound("GetRepositoryByID", nil)
		} else {
			ctx.ServerError("GetRepositoryByID", err)
		}
		return nil
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil
	}

	if !perm.CanRead(unit.TypeCode) {
		log.Trace("Permission Denied: User %-v cannot read %-v of repo %-v\n"+
			"User in repo has Permissions: %-+v",
			ctx.Doer,
			unit.TypeCode,
			ctx.Repo,
			perm)
		ctx.NotFound("getRepository", nil)
		return nil
	}
	return repo
}

func getPullInfo(ctx *context.Context) (issue *issues_model.Issue, ok bool) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
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
		ctx.NotFound("ViewPullCommits", nil)
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

func setMergeTarget(ctx *context.Context, pull *issues_model.PullRequest) {
	if ctx.Repo.Owner.Name == pull.MustHeadUserName(ctx) {
		ctx.Data["HeadTarget"] = pull.HeadBranch
	} else if pull.HeadRepo == nil {
		ctx.Data["HeadTarget"] = pull.MustHeadUserName(ctx) + ":" + pull.HeadBranch
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
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest

	mergeBaseCommitID := GetMergedBaseCommitID(ctx, issue)

	if mergeBaseCommitID == "" {
		ctx.NotFound("PullFiles", nil)
		return
	}

	headCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(pull.GetGitRefName())
	if err != nil {
		ctx.ServerError("GetRefCommitID", err)
		return
	}

	diffOptions := &gitdiff.DiffOptions{
		BeforeCommitID:     mergeBaseCommitID,
		AfterCommitID:      headCommitID,
		MaxLines:           setting.Git.MaxGitDiffLines,
		MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:           setting.Git.MaxGitDiffFiles,
		WhitespaceBehavior: gitdiff.GetWhitespaceFlag(ctx.Data["WhitespaceBehavior"].(string)),
	}

	diff, err := gitdiff.GetPullDiffStats(ctx.Repo.GitRepo, diffOptions)
	if err != nil {
		ctx.ServerError("GetPullDiffStats", err)
		return
	}

	ctx.Data["Diff"] = diff
}

func GetMergedBaseCommitID(ctx *context.Context, issue *issues_model.Issue) string {
	pull := issue.PullRequest

	var baseCommit string
	// Some migrated PR won't have any Base SHA and lose history, try to get one
	if pull.MergeBase == "" {
		var commitSHA, parentCommit string
		// If there is a head or a patch file, and it is readable, grab info
		commitSHA, err := ctx.Repo.GitRepo.GetRefCommitID(pull.GetGitRefName())
		if err != nil {
			// Head File does not exist, try the patch
			commitSHA, err = ctx.Repo.GitRepo.ReadPatchCommit(pull.Index)
			if err == nil {
				// Recreate pull head in files for next time
				if err := ctx.Repo.GitRepo.SetReference(pull.GetGitRefName(), commitSHA); err != nil {
					log.Error("Could not write head file", err)
				}
			} else {
				// There is no history available
				log.Trace("No history file available for PR %d", pull.Index)
			}
		}
		if commitSHA != "" {
			// Get immediate parent of the first commit in the patch, grab history back
			parentCommit, _, err = git.NewCommand(ctx, "rev-list", "-1", "--skip=1").AddDynamicArguments(commitSHA).RunStdString(&git.RunOpts{Dir: ctx.Repo.GitRepo.Path})
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

func preparePullViewPullInfo(ctx *context.Context, issue *issues_model.Issue) *git.CompareInfo {
	if !issue.IsPull {
		return nil
	}
	if issue.PullRequest.HasMerged {
		return prepareMergedViewPullInfo(ctx, issue)
	}
	return prepareViewPullInfo(ctx, issue)
}

// prepareMergedViewPullInfo show meta information for a merged pull request view page
func prepareMergedViewPullInfo(ctx *context.Context, issue *issues_model.Issue) *git.CompareInfo {
	pull := issue.PullRequest

	setMergeTarget(ctx, pull)
	ctx.Data["HasMerged"] = true

	baseCommit := GetMergedBaseCommitID(ctx, issue)

	compareInfo, err := ctx.Repo.GitRepo.GetCompareInfo(ctx.Repo.Repository.RepoPath(),
		baseCommit, pull.GetGitRefName(), false, false)
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Not a valid object name") || strings.Contains(err.Error(), "unknown revision or path not in the working tree") {
			ctx.Data["IsPullRequestBroken"] = true
			ctx.Data["BaseTarget"] = pull.BaseBranch
			ctx.Data["NumCommits"] = 0
			ctx.Data["NumFiles"] = 0
			return nil
		}

		ctx.ServerError("GetCompareInfo", err)
		return nil
	}
	ctx.Data["NumCommits"] = len(compareInfo.Commits)
	ctx.Data["NumFiles"] = compareInfo.NumFiles

	if len(compareInfo.Commits) != 0 {
		sha := compareInfo.Commits[0].ID.String()
		commitStatuses, _, err := git_model.GetLatestCommitStatus(ctx, ctx.Repo.Repository.ID, sha, db.ListOptionsAll)
		if err != nil {
			ctx.ServerError("GetLatestCommitStatus", err)
			return nil
		}
		if !ctx.Repo.CanRead(unit.TypeActions) {
			git_model.CommitStatusesHideActionsURL(ctx, commitStatuses)
		}

		if len(commitStatuses) != 0 {
			ctx.Data["LatestCommitStatuses"] = commitStatuses
			ctx.Data["LatestCommitStatus"] = git_model.CalcCommitStatus(commitStatuses)
		}
	}

	return compareInfo
}

// prepareViewPullInfo show meta information for a pull request preview page
func prepareViewPullInfo(ctx *context.Context, issue *issues_model.Issue) *git.CompareInfo {
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes

	repo := ctx.Repo.Repository
	pull := issue.PullRequest

	if err := pull.LoadHeadRepo(ctx); err != nil {
		ctx.ServerError("LoadHeadRepo", err)
		return nil
	}

	if err := pull.LoadBaseRepo(ctx); err != nil {
		ctx.ServerError("LoadBaseRepo", err)
		return nil
	}

	setMergeTarget(ctx, pull)

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repo.ID, pull.BaseBranch)
	if err != nil {
		ctx.ServerError("LoadProtectedBranch", err)
		return nil
	}
	ctx.Data["EnableStatusCheck"] = pb != nil && pb.EnableStatusCheck

	var baseGitRepo *git.Repository
	if pull.BaseRepoID == ctx.Repo.Repository.ID && ctx.Repo.GitRepo != nil {
		baseGitRepo = ctx.Repo.GitRepo
	} else {
		baseGitRepo, err := gitrepo.OpenRepository(ctx, pull.BaseRepo)
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return nil
		}
		defer baseGitRepo.Close()
	}

	if !baseGitRepo.IsBranchExist(pull.BaseBranch) {
		ctx.Data["BaseBranchNotExist"] = true
		ctx.Data["IsPullRequestBroken"] = true
		ctx.Data["BaseTarget"] = pull.BaseBranch
		ctx.Data["HeadTarget"] = pull.HeadBranch

		sha, err := baseGitRepo.GetRefCommitID(pull.GetGitRefName())
		if err != nil {
			ctx.ServerError(fmt.Sprintf("GetRefCommitID(%s)", pull.GetGitRefName()), err)
			return nil
		}
		commitStatuses, _, err := git_model.GetLatestCommitStatus(ctx, repo.ID, sha, db.ListOptionsAll)
		if err != nil {
			ctx.ServerError("GetLatestCommitStatus", err)
			return nil
		}
		if !ctx.Repo.CanRead(unit.TypeActions) {
			git_model.CommitStatusesHideActionsURL(ctx, commitStatuses)
		}

		if len(commitStatuses) > 0 {
			ctx.Data["LatestCommitStatuses"] = commitStatuses
			ctx.Data["LatestCommitStatus"] = git_model.CalcCommitStatus(commitStatuses)
		}

		compareInfo, err := baseGitRepo.GetCompareInfo(pull.BaseRepo.RepoPath(),
			pull.MergeBase, pull.GetGitRefName(), false, false)
		if err != nil {
			if strings.Contains(err.Error(), "fatal: Not a valid object name") {
				ctx.Data["IsPullRequestBroken"] = true
				ctx.Data["BaseTarget"] = pull.BaseBranch
				ctx.Data["NumCommits"] = 0
				ctx.Data["NumFiles"] = 0
				return nil
			}

			ctx.ServerError("GetCompareInfo", err)
			return nil
		}

		ctx.Data["NumCommits"] = len(compareInfo.Commits)
		ctx.Data["NumFiles"] = compareInfo.NumFiles
		return compareInfo
	}

	var headBranchExist bool
	var headBranchSha string
	// HeadRepo may be missing
	if pull.HeadRepo != nil {
		headGitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pull.HeadRepo)
		if err != nil {
			ctx.ServerError("RepositoryFromContextOrOpen", err)
			return nil
		}
		defer closer.Close()

		if pull.Flow == issues_model.PullRequestFlowGithub {
			headBranchExist = headGitRepo.IsBranchExist(pull.HeadBranch)
		} else {
			headBranchExist = git.IsReferenceExist(ctx, baseGitRepo.Path, pull.GetGitRefName())
		}

		if headBranchExist {
			if pull.Flow != issues_model.PullRequestFlowGithub {
				headBranchSha, err = baseGitRepo.GetRefCommitID(pull.GetGitRefName())
			} else {
				headBranchSha, err = headGitRepo.GetBranchCommitID(pull.HeadBranch)
			}
			if err != nil {
				ctx.ServerError("GetBranchCommitID", err)
				return nil
			}
		}
	}

	if headBranchExist {
		var err error
		ctx.Data["UpdateAllowed"], ctx.Data["UpdateByRebaseAllowed"], err = pull_service.IsUserAllowedToUpdate(ctx, pull, ctx.Doer)
		if err != nil {
			ctx.ServerError("IsUserAllowedToUpdate", err)
			return nil
		}
		ctx.Data["GetCommitMessages"] = pull_service.GetSquashMergeCommitMessages(ctx, pull)
	} else {
		ctx.Data["GetCommitMessages"] = ""
	}

	sha, err := baseGitRepo.GetRefCommitID(pull.GetGitRefName())
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.Data["IsPullRequestBroken"] = true
			if pull.IsSameRepo() {
				ctx.Data["HeadTarget"] = pull.HeadBranch
			} else if pull.HeadRepo == nil {
				ctx.Data["HeadTarget"] = ctx.Locale.Tr("repo.pull.deleted_branch", pull.HeadBranch)
			} else {
				ctx.Data["HeadTarget"] = pull.HeadRepo.OwnerName + ":" + pull.HeadBranch
			}
			ctx.Data["BaseTarget"] = pull.BaseBranch
			ctx.Data["NumCommits"] = 0
			ctx.Data["NumFiles"] = 0
			return nil
		}
		ctx.ServerError(fmt.Sprintf("GetRefCommitID(%s)", pull.GetGitRefName()), err)
		return nil
	}

	commitStatuses, _, err := git_model.GetLatestCommitStatus(ctx, repo.ID, sha, db.ListOptionsAll)
	if err != nil {
		ctx.ServerError("GetLatestCommitStatus", err)
		return nil
	}
	if !ctx.Repo.CanRead(unit.TypeActions) {
		git_model.CommitStatusesHideActionsURL(ctx, commitStatuses)
	}

	if len(commitStatuses) > 0 {
		ctx.Data["LatestCommitStatuses"] = commitStatuses
		ctx.Data["LatestCommitStatus"] = git_model.CalcCommitStatus(commitStatuses)
	}

	if pb != nil && pb.EnableStatusCheck {
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
		ctx.Data["MissingRequiredChecks"] = missingRequiredChecks

		ctx.Data["is_context_required"] = func(context string) bool {
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
		ctx.Data["RequiredStatusCheckState"] = pull_service.MergeRequiredContextsCommitStatus(commitStatuses, pb.StatusCheckContexts)
	}

	ctx.Data["HeadBranchMovedOn"] = headBranchSha != sha
	ctx.Data["HeadBranchCommitID"] = headBranchSha
	ctx.Data["PullHeadCommitID"] = sha

	if pull.HeadRepo == nil || !headBranchExist || (!pull.Issue.IsClosed && (headBranchSha != sha)) {
		ctx.Data["IsPullRequestBroken"] = true
		if pull.IsSameRepo() {
			ctx.Data["HeadTarget"] = pull.HeadBranch
		} else if pull.HeadRepo == nil {
			ctx.Data["HeadTarget"] = ctx.Locale.Tr("repo.pull.deleted_branch", pull.HeadBranch)
		} else {
			ctx.Data["HeadTarget"] = pull.HeadRepo.OwnerName + ":" + pull.HeadBranch
		}
	}

	compareInfo, err := baseGitRepo.GetCompareInfo(pull.BaseRepo.RepoPath(),
		git.BranchPrefix+pull.BaseBranch, pull.GetGitRefName(), false, false)
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Not a valid object name") {
			ctx.Data["IsPullRequestBroken"] = true
			ctx.Data["BaseTarget"] = pull.BaseBranch
			ctx.Data["NumCommits"] = 0
			ctx.Data["NumFiles"] = 0
			return nil
		}

		ctx.ServerError("GetCompareInfo", err)
		return nil
	}

	if compareInfo.HeadCommitID == compareInfo.MergeBase {
		ctx.Data["IsNothingToCompare"] = true
	}

	if pull.IsWorkInProgress(ctx) {
		ctx.Data["IsPullWorkInProgress"] = true
		ctx.Data["WorkInProgressPrefix"] = pull.GetWorkInProgressPrefix(ctx)
	}

	if pull.IsFilesConflicted() {
		ctx.Data["IsPullFilesConflicted"] = true
		ctx.Data["ConflictedFiles"] = pull.ConflictedFiles
	}

	ctx.Data["NumCommits"] = len(compareInfo.Commits)
	ctx.Data["NumFiles"] = compareInfo.NumFiles
	return compareInfo
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

	commits, lastReviewCommitSha, err := pull_service.GetPullCommits(ctx, issue)
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

	prInfo := preparePullViewPullInfo(ctx, issue)
	if ctx.Written() {
		return
	} else if prInfo == nil {
		ctx.NotFound("ViewPullCommits", nil)
		return
	}

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name

	commits := processGitCommits(ctx, prInfo.Commits)
	ctx.Data["Commits"] = commits
	ctx.Data["CommitCount"] = len(commits)

	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)
	ctx.Data["IsIssuePoster"] = ctx.IsSigned && issue.IsPoster(ctx.Doer.ID)

	// For PR commits page
	PrepareBranchList(ctx)
	if ctx.Written() {
		return
	}
	getBranchData(ctx, issue)
	ctx.HTML(http.StatusOK, tplPullCommits)
}

// ViewPullFiles render pull request changed files list page
func viewPullFiles(ctx *context.Context, specifiedStartCommit, specifiedEndCommit string, willShowSpecifiedCommitRange, willShowSpecifiedCommit bool) {
	ctx.Data["PageIsPullList"] = true
	ctx.Data["PageIsPullFiles"] = true

	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest

	var (
		startCommitID string
		endCommitID   string
		gitRepo       = ctx.Repo.GitRepo
	)

	prInfo := preparePullViewPullInfo(ctx, issue)
	if ctx.Written() {
		return
	} else if prInfo == nil {
		ctx.NotFound("ViewPullFiles", nil)
		return
	}

	// Validate the given commit sha to show (if any passed)
	if willShowSpecifiedCommit || willShowSpecifiedCommitRange {
		foundStartCommit := len(specifiedStartCommit) == 0
		foundEndCommit := len(specifiedEndCommit) == 0

		if !(foundStartCommit && foundEndCommit) {
			for _, commit := range prInfo.Commits {
				if commit.ID.String() == specifiedStartCommit {
					foundStartCommit = true
				}
				if commit.ID.String() == specifiedEndCommit {
					foundEndCommit = true
				}

				if foundStartCommit && foundEndCommit {
					break
				}
			}
		}

		if !(foundStartCommit && foundEndCommit) {
			ctx.NotFound("Given SHA1 not found for this PR", nil)
			return
		}
	}

	if ctx.Written() {
		return
	}

	headCommitID, err := gitRepo.GetRefCommitID(pull.GetGitRefName())
	if err != nil {
		ctx.ServerError("GetRefCommitID", err)
		return
	}

	ctx.Data["IsShowingOnlySingleCommit"] = willShowSpecifiedCommit

	if willShowSpecifiedCommit || willShowSpecifiedCommitRange {
		if len(specifiedEndCommit) > 0 {
			endCommitID = specifiedEndCommit
		} else {
			endCommitID = headCommitID
		}
		if len(specifiedStartCommit) > 0 {
			startCommitID = specifiedStartCommit
		} else {
			startCommitID = prInfo.MergeBase
		}
		ctx.Data["IsShowingAllCommits"] = false
	} else {
		endCommitID = headCommitID
		startCommitID = prInfo.MergeBase
		ctx.Data["IsShowingAllCommits"] = true
	}

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["AfterCommitID"] = endCommitID
	ctx.Data["BeforeCommitID"] = startCommitID

	fileOnly := ctx.FormBool("file-only")

	maxLines, maxFiles := setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles
	files := ctx.FormStrings("files")
	if fileOnly && (len(files) == 2 || len(files) == 1) {
		maxLines, maxFiles = -1, -1
	}

	diffOptions := &gitdiff.DiffOptions{
		AfterCommitID:      endCommitID,
		SkipTo:             ctx.FormString("skip-to"),
		MaxLines:           maxLines,
		MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:           maxFiles,
		WhitespaceBehavior: gitdiff.GetWhitespaceFlag(ctx.Data["WhitespaceBehavior"].(string)),
		FileOnly:           fileOnly,
	}

	if !willShowSpecifiedCommit {
		diffOptions.BeforeCommitID = startCommitID
	}

	var methodWithError string
	var diff *gitdiff.Diff

	// if we're not logged in or only a single commit (or commit range) is shown we
	// have to load only the diff and not get the viewed information
	// as the viewed information is designed to be loaded only on latest PR
	// diff and if you're signed in.
	if !ctx.IsSigned || willShowSpecifiedCommit || willShowSpecifiedCommitRange {
		diff, err = gitdiff.GetDiff(ctx, gitRepo, diffOptions, files...)
		methodWithError = "GetDiff"
	} else {
		diff, err = gitdiff.SyncAndGetUserSpecificDiff(ctx, ctx.Doer.ID, pull, gitRepo, diffOptions, files...)
		methodWithError = "SyncAndGetUserSpecificDiff"
	}
	if err != nil {
		ctx.ServerError(methodWithError, err)
		return
	}

	ctx.PageData["prReview"] = map[string]any{
		"numberOfFiles":       diff.NumFiles,
		"numberOfViewedFiles": diff.NumViewedFiles,
	}

	if err = diff.LoadComments(ctx, issue, ctx.Doer, ctx.Data["ShowOutdatedComments"].(bool)); err != nil {
		ctx.ServerError("LoadComments", err)
		return
	}

	for _, file := range diff.Files {
		for _, section := range file.Sections {
			for _, line := range section.Lines {
				for _, comment := range line.Comments {
					if err := comment.LoadAttachments(ctx); err != nil {
						ctx.ServerError("LoadAttachments", err)
						return
					}
				}
			}
		}
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pull.BaseRepoID, pull.BaseBranch)
	if err != nil {
		ctx.ServerError("LoadProtectedBranch", err)
		return
	}

	if pb != nil {
		glob := pb.GetProtectedFilePatterns()
		if len(glob) != 0 {
			for _, file := range diff.Files {
				file.IsProtected = pb.IsProtectedFile(glob, file.Name)
			}
		}
	}

	ctx.Data["Diff"] = diff
	ctx.Data["DiffNotAvailable"] = diff.NumFiles == 0

	baseCommit, err := ctx.Repo.GitRepo.GetCommit(startCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return
	}
	commit, err := gitRepo.GetCommit(endCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return
	}

	if ctx.IsSigned && ctx.Doer != nil {
		if ctx.Data["CanMarkConversation"], err = issues_model.CanMarkConversation(ctx, issue, ctx.Doer); err != nil {
			ctx.ServerError("CanMarkConversation", err)
			return
		}
	}

	setCompareContext(ctx, baseCommit, commit, ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)

	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	handleMentionableAssigneesAndTeams(ctx, shared_user.MakeSelfOnTop(ctx.Doer, assigneeUsers))
	if ctx.Written() {
		return
	}

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

	getBranchData(ctx, issue)
	ctx.Data["IsIssuePoster"] = ctx.IsSigned && issue.IsPoster(ctx.Doer.ID)
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
	if !willShowSpecifiedCommit && !willShowSpecifiedCommitRange && pull.Flow == issues_model.PullRequestFlowGithub {
		if err := pull.LoadHeadRepo(ctx); err != nil {
			ctx.ServerError("LoadHeadRepo", err)
			return
		}

		if pull.HeadRepo != nil {
			if !pull.HasMerged && ctx.Doer != nil {
				perm, err := access_model.GetUserRepoPermission(ctx, pull.HeadRepo, ctx.Doer)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
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
	viewPullFiles(ctx, "", ctx.PathParam("sha"), true, true)
}

func ViewPullFilesForRange(ctx *context.Context) {
	viewPullFiles(ctx, ctx.PathParam("shaFrom"), ctx.PathParam("shaTo"), true, false)
}

func ViewPullFilesStartingFromCommit(ctx *context.Context) {
	viewPullFiles(ctx, "", ctx.PathParam("sha"), true, false)
}

func ViewPullFilesForAllCommitsOfPr(ctx *context.Context) {
	viewPullFiles(ctx, "", "", false, false)
}

// UpdatePullRequest merge PR's baseBranch into headBranch
func UpdatePullRequest(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	if issue.IsClosed {
		ctx.NotFound("MergePullRequest", nil)
		return
	}
	if issue.PullRequest.HasMerged {
		ctx.NotFound("MergePullRequest", nil)
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

	if err = pull_service.Update(ctx, issue.PullRequest, ctx.Doer, message, rebase); err != nil {
		if pull_service.IsErrMergeConflicts(err) {
			conflictError := err.(pull_service.ErrMergeConflicts)
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.pulls.merge_conflict"),
				"Summary": ctx.Tr("repo.pulls.merge_conflict_summary"),
				"Details": utils.SanitizeFlashErrorString(conflictError.StdErr) + "<br>" + utils.SanitizeFlashErrorString(conflictError.StdOut),
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
				"Message": ctx.Tr("repo.pulls.rebase_conflict", utils.SanitizeFlashErrorString(conflictError.CommitSHA)),
				"Summary": ctx.Tr("repo.pulls.rebase_conflict_summary"),
				"Details": utils.SanitizeFlashErrorString(conflictError.StdErr) + "<br>" + utils.SanitizeFlashErrorString(conflictError.StdOut),
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
	if err := pull_service.CheckPullMergeable(ctx, ctx.Doer, &ctx.Repo.Permission, pr, mergeCheckType, form.ForceMerge); err != nil {
		switch {
		case errors.Is(err, pull_service.ErrIsClosed):
			if issue.IsPull {
				ctx.JSONError(ctx.Tr("repo.pulls.is_closed"))
			} else {
				ctx.JSONError(ctx.Tr("repo.issues.closed_title"))
			}
		case errors.Is(err, pull_service.ErrUserNotAllowedToMerge):
			ctx.JSONError(ctx.Tr("repo.pulls.update_not_allowed"))
		case errors.Is(err, pull_service.ErrHasMerged):
			ctx.JSONError(ctx.Tr("repo.pulls.has_merged"))
		case errors.Is(err, pull_service.ErrIsWorkInProgress):
			ctx.JSONError(ctx.Tr("repo.pulls.no_merge_wip"))
		case errors.Is(err, pull_service.ErrNotMergeableState):
			ctx.JSONError(ctx.Tr("repo.pulls.no_merge_not_ready"))
		case pull_service.IsErrDisallowedToMerge(err):
			ctx.JSONError(ctx.Tr("repo.pulls.no_merge_not_ready"))
		case asymkey_service.IsErrWontSign(err):
			ctx.JSONError(err.Error()) // has no translation ...
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

	if form.MergeWhenChecksSucceed {
		// delete all scheduled auto merges
		_ = pull_model.DeleteScheduledAutoMerge(ctx, pr.ID)
		// schedule auto merge
		scheduled, err := automerge.ScheduleAutoMerge(ctx, ctx.Doer, pr, repo_model.MergeStyle(form.Do), message, form.DeleteBranchAfterMerge)
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

	if err := pull_service.Merge(ctx, pr, ctx.Doer, ctx.Repo.GitRepo, repo_model.MergeStyle(form.Do), form.HeadCommitID, message, false); err != nil {
		if pull_service.IsErrInvalidMergeStyle(err) {
			ctx.JSONError(ctx.Tr("repo.pulls.invalid_merge_option"))
		} else if pull_service.IsErrMergeConflicts(err) {
			conflictError := err.(pull_service.ErrMergeConflicts)
			flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
				"Message": ctx.Tr("repo.editor.merge_conflict"),
				"Summary": ctx.Tr("repo.editor.merge_conflict_summary"),
				"Details": utils.SanitizeFlashErrorString(conflictError.StdErr) + "<br>" + utils.SanitizeFlashErrorString(conflictError.StdOut),
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
				"Message": ctx.Tr("repo.pulls.rebase_conflict", utils.SanitizeFlashErrorString(conflictError.CommitSHA)),
				"Summary": ctx.Tr("repo.pulls.rebase_conflict_summary"),
				"Details": utils.SanitizeFlashErrorString(conflictError.StdErr) + "<br>" + utils.SanitizeFlashErrorString(conflictError.StdOut),
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
					"Details": utils.SanitizeFlashErrorString(pushrejErr.Message),
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

	if !form.DeleteBranchAfterMerge {
		ctx.JSONRedirect(issue.Link())
		return
	}

	// Don't cleanup when other pr use this branch as head branch
	exist, err := issues_model.HasUnmergedPullRequestsByHeadInfo(ctx, pr.HeadRepoID, pr.HeadBranch)
	if err != nil {
		ctx.ServerError("HasUnmergedPullRequestsByHeadInfo", err)
		return
	}
	if exist {
		ctx.JSONRedirect(issue.Link())
		return
	}

	var headRepo *git.Repository
	if ctx.Repo != nil && ctx.Repo.Repository != nil && pr.HeadRepoID == ctx.Repo.Repository.ID && ctx.Repo.GitRepo != nil {
		headRepo = ctx.Repo.GitRepo
	} else {
		headRepo, err = gitrepo.OpenRepository(ctx, pr.HeadRepo)
		if err != nil {
			ctx.ServerError(fmt.Sprintf("OpenRepository[%s]", pr.HeadRepo.FullName()), err)
			return
		}
		defer headRepo.Close()
	}
	deleteBranch(ctx, pr, headRepo)
	ctx.JSONRedirect(issue.Link())
}

// CancelAutoMergePullRequest cancels a scheduled pr
func CancelAutoMergePullRequest(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
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
	if issues_model.StopwatchExists(ctx, user.ID, issue.ID) {
		if err := issues_model.CreateOrStopIssueStopwatch(ctx, user, issue); err != nil {
			return err
		}
	}

	return nil
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
	defer func() {
		if ci != nil && ci.HeadGitRepo != nil {
			ci.HeadGitRepo.Close()
		}
	}()
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
		HeadBranch:          ci.HeadBranch,
		BaseBranch:          ci.BaseBranch,
		HeadRepo:            ci.HeadRepo,
		BaseRepo:            repo,
		MergeBase:           ci.CompareInfo.MergeBase,
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
	}
	if err := pull_service.NewPullRequest(ctx, prOpts); err != nil {
		switch {
		case repo_model.IsErrUserDoesNotHaveAccessToRepo(err):
			ctx.Error(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err.Error())
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
				"Details": utils.SanitizeFlashErrorString(pushrejErr.Message),
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

	if projectID > 0 && ctx.Repo.CanWrite(unit.TypeProjects) {
		if err := issues_model.IssueAssignOrRemoveProject(ctx, pullIssue, ctx.Doer, projectID, 0); err != nil {
			if !errors.Is(err, util.ErrPermissionDenied) {
				ctx.ServerError("IssueAssignOrRemoveProject", err)
				return
			}
		}
	}

	log.Trace("Pull request created: %d/%d", repo.ID, pullIssue.ID)
	ctx.JSONRedirect(pullIssue.Link())
}

// CleanUpPullRequest responses for delete merged branch when PR has been merged
func CleanUpPullRequest(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}

	pr := issue.PullRequest

	// Don't cleanup unmerged and unclosed PRs and agit PRs
	if !pr.HasMerged && !issue.IsClosed && pr.Flow != issues_model.PullRequestFlowGithub {
		ctx.NotFound("CleanUpPullRequest", nil)
		return
	}

	// Don't cleanup when there are other PR's that use this branch as head branch.
	exist, err := issues_model.HasUnmergedPullRequestsByHeadInfo(ctx, pr.HeadRepoID, pr.HeadBranch)
	if err != nil {
		ctx.ServerError("HasUnmergedPullRequestsByHeadInfo", err)
		return
	}
	if exist {
		ctx.NotFound("CleanUpPullRequest", nil)
		return
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		ctx.ServerError("LoadHeadRepo", err)
		return
	} else if pr.HeadRepo == nil {
		// Forked repository has already been deleted
		ctx.NotFound("CleanUpPullRequest", nil)
		return
	} else if err = pr.LoadBaseRepo(ctx); err != nil {
		ctx.ServerError("LoadBaseRepo", err)
		return
	} else if err = pr.HeadRepo.LoadOwner(ctx); err != nil {
		ctx.ServerError("HeadRepo.LoadOwner", err)
		return
	}

	if err := repo_service.CanDeleteBranch(ctx, pr.HeadRepo, pr.HeadBranch, ctx.Doer); err != nil {
		if errors.Is(err, util.ErrPermissionDenied) {
			ctx.NotFound("CanDeleteBranch", nil)
		} else {
			ctx.ServerError("CanDeleteBranch", err)
		}
		return
	}

	fullBranchName := pr.HeadRepo.Owner.Name + "/" + pr.HeadBranch

	var gitBaseRepo *git.Repository

	// Assume that the base repo is the current context (almost certainly)
	if ctx.Repo != nil && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID == pr.BaseRepoID && ctx.Repo.GitRepo != nil {
		gitBaseRepo = ctx.Repo.GitRepo
	} else {
		// If not just open it
		gitBaseRepo, err = gitrepo.OpenRepository(ctx, pr.BaseRepo)
		if err != nil {
			ctx.ServerError(fmt.Sprintf("OpenRepository[%s]", pr.BaseRepo.FullName()), err)
			return
		}
		defer gitBaseRepo.Close()
	}

	// Now assume that the head repo is the same as the base repo (reasonable chance)
	gitRepo := gitBaseRepo
	// But if not: is it the same as the context?
	if pr.BaseRepoID != pr.HeadRepoID && ctx.Repo != nil && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID == pr.HeadRepoID && ctx.Repo.GitRepo != nil {
		gitRepo = ctx.Repo.GitRepo
	} else if pr.BaseRepoID != pr.HeadRepoID {
		// Otherwise just load it up
		gitRepo, err = gitrepo.OpenRepository(ctx, pr.HeadRepo)
		if err != nil {
			ctx.ServerError(fmt.Sprintf("OpenRepository[%s]", pr.HeadRepo.FullName()), err)
			return
		}
		defer gitRepo.Close()
	}

	defer func() {
		ctx.JSONRedirect(issue.Link())
	}()

	// Check if branch has no new commits
	headCommitID, err := gitBaseRepo.GetRefCommitID(pr.GetGitRefName())
	if err != nil {
		log.Error("GetRefCommitID: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}
	branchCommitID, err := gitRepo.GetBranchCommitID(pr.HeadBranch)
	if err != nil {
		log.Error("GetBranchCommitID: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}
	if headCommitID != branchCommitID {
		ctx.Flash.Error(ctx.Tr("repo.branch.delete_branch_has_new_commits", fullBranchName))
		return
	}

	deleteBranch(ctx, pr, gitRepo)
}

func deleteBranch(ctx *context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository) {
	fullBranchName := pr.HeadRepo.FullName() + ":" + pr.HeadBranch

	if err := repo_service.DeleteBranch(ctx, ctx.Doer, pr.HeadRepo, gitRepo, pr.HeadBranch, pr); err != nil {
		switch {
		case git.IsErrBranchNotExist(err):
			ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		case errors.Is(err, repo_service.ErrBranchIsDefault):
			ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		case errors.Is(err, git_model.ErrBranchIsProtected):
			ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		default:
			log.Error("DeleteBranch: %v", err)
			ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.deletion_success", fullBranchName))
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
			ctx.NotFound("GetPullRequestByIndex", err)
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
		ctx.Error(http.StatusNotFound)
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(http.StatusForbidden)
		return
	}

	targetBranch := ctx.FormTrim("target_branch")
	if len(targetBranch) == 0 {
		ctx.Error(http.StatusNoContent)
		return
	}

	if err := pull_service.ChangeTargetBranch(ctx, pr, ctx.Doer, targetBranch); err != nil {
		if issues_model.IsErrPullRequestAlreadyExists(err) {
			err := err.(issues_model.ErrPullRequestAlreadyExists)

			RepoRelPath := ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name
			errorMessage := ctx.Tr("repo.pulls.has_pull_request", html.EscapeString(ctx.Repo.RepoLink+"/pulls/"+strconv.FormatInt(err.IssueID, 10)), html.EscapeString(RepoRelPath), err.IssueID) // FIXME: Creates url inside locale string

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusConflict, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		} else if issues_model.IsErrIssueIsClosed(err) {
			errorMessage := ctx.Tr("repo.pulls.is_closed")

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusConflict, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		} else if pull_service.IsErrPullRequestHasMerged(err) {
			errorMessage := ctx.Tr("repo.pulls.has_merged")

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusConflict, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		} else if git_model.IsErrBranchesEqual(err) {
			errorMessage := ctx.Tr("repo.pulls.nothing_to_compare")

			ctx.Flash.Error(errorMessage)
			ctx.JSON(http.StatusBadRequest, map[string]any{
				"error":      err.Error(),
				"user_error": errorMessage,
			})
		} else {
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
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.ServerError("GetPullRequestByIndex", err)
		}
		return
	}

	if err := pull_service.SetAllowEdits(ctx, ctx.Doer, pr, form.AllowMaintainerEdit); err != nil {
		if errors.Is(err, pull_service.ErrUserHasNoPermissionForAction) {
			ctx.Error(http.StatusForbidden)
			return
		}
		ctx.ServerError("SetAllowEdits", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"allow_maintainer_edit": pr.AllowMaintainerEdit,
	})
}
