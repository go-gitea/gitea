// Copyright 2018 The Gitea Authors.
// Copyright 2014 The Gogs Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"container/list"
	"fmt"
	"io"
	"path"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
)

const (
	tplFork        base.TplName = "repo/pulls/fork"
	tplComparePull base.TplName = "repo/pulls/compare"
	tplPullCommits base.TplName = "repo/pulls/commits"
	tplPullFiles   base.TplName = "repo/pulls/files"

	pullRequestTemplateKey = "PullRequestTemplate"
)

var (
	pullRequestTemplateCandidates = []string{
		"PULL_REQUEST_TEMPLATE.md",
		"pull_request_template.md",
		".gitea/PULL_REQUEST_TEMPLATE.md",
		".gitea/pull_request_template.md",
		".github/PULL_REQUEST_TEMPLATE.md",
		".github/pull_request_template.md",
	}
)

func getForkRepository(ctx *context.Context) *models.Repository {
	forkRepo, err := models.GetRepositoryByID(ctx.ParamsInt64(":repoid"))
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.NotFound("GetRepositoryByID", nil)
		} else {
			ctx.ServerError("GetRepositoryByID", err)
		}
		return nil
	}

	perm, err := models.GetUserRepoPermission(forkRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil
	}

	if forkRepo.IsEmpty || !perm.CanRead(models.UnitTypeCode) {
		ctx.NotFound("getForkRepository", nil)
		return nil
	}

	ctx.Data["repo_name"] = forkRepo.Name
	ctx.Data["description"] = forkRepo.Description
	ctx.Data["IsPrivate"] = forkRepo.IsPrivate
	canForkToUser := forkRepo.OwnerID != ctx.User.ID && !ctx.User.HasForkedRepo(forkRepo.ID)

	if err = forkRepo.GetOwner(); err != nil {
		ctx.ServerError("GetOwner", err)
		return nil
	}
	ctx.Data["ForkFrom"] = forkRepo.Owner.Name + "/" + forkRepo.Name
	ctx.Data["ForkFromOwnerID"] = forkRepo.Owner.ID

	if err := ctx.User.GetOwnedOrganizations(); err != nil {
		ctx.ServerError("GetOwnedOrganizations", err)
		return nil
	}
	var orgs []*models.User
	for _, org := range ctx.User.OwnedOrgs {
		if forkRepo.OwnerID != org.ID && !org.HasForkedRepo(forkRepo.ID) {
			orgs = append(orgs, org)
		}
	}

	var traverseParentRepo = forkRepo
	for {
		if ctx.User.ID == traverseParentRepo.OwnerID {
			canForkToUser = false
		} else {
			for i, org := range orgs {
				if org.ID == traverseParentRepo.OwnerID {
					orgs = append(orgs[:i], orgs[i+1:]...)
					break
				}
			}
		}

		if !traverseParentRepo.IsFork {
			break
		}
		traverseParentRepo, err = models.GetRepositoryByID(traverseParentRepo.ForkID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return nil
		}
	}

	ctx.Data["CanForkToUser"] = canForkToUser
	ctx.Data["Orgs"] = orgs

	if canForkToUser {
		ctx.Data["ContextUser"] = ctx.User
	} else if len(orgs) > 0 {
		ctx.Data["ContextUser"] = orgs[0]
	}

	return forkRepo
}

// Fork render repository fork page
func Fork(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_fork")

	getForkRepository(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(200, tplFork)
}

// ForkPost response for forking a repository
func ForkPost(ctx *context.Context, form auth.CreateRepoForm) {
	ctx.Data["Title"] = ctx.Tr("new_fork")

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}

	forkRepo := getForkRepository(ctx)
	if ctx.Written() {
		return
	}

	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(200, tplFork)
		return
	}

	var err error
	var traverseParentRepo = forkRepo
	for {
		if ctxUser.ID == traverseParentRepo.OwnerID {
			ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplFork, &form)
			return
		}
		repo, has := models.HasForkedRepo(ctxUser.ID, traverseParentRepo.ID)
		if has {
			ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + repo.Name)
			return
		}
		if !traverseParentRepo.IsFork {
			break
		}
		traverseParentRepo, err = models.GetRepositoryByID(traverseParentRepo.ForkID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
	}

	// Check ownership of organization.
	if ctxUser.IsOrganization() {
		isOwner, err := ctxUser.IsOwnedBy(ctx.User.ID)
		if err != nil {
			ctx.ServerError("IsOwnedBy", err)
			return
		} else if !isOwner {
			ctx.Error(403)
			return
		}
	}

	repo, err := models.ForkRepository(ctx.User, ctxUser, forkRepo, form.RepoName, form.Description)
	if err != nil {
		ctx.Data["Err_RepoName"] = true
		switch {
		case models.IsErrRepoAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplFork, &form)
		case models.IsErrNameReserved(err):
			ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(models.ErrNameReserved).Name), tplFork, &form)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tplFork, &form)
		default:
			ctx.ServerError("ForkPost", err)
		}
		return
	}

	log.Trace("Repository forked[%d]: %s/%s", forkRepo.ID, ctxUser.Name, repo.Name)
	ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + repo.Name)
}

func checkPullInfo(ctx *context.Context) *models.Issue {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return nil
	}
	if err = issue.LoadPoster(); err != nil {
		ctx.ServerError("LoadPoster", err)
		return nil
	}
	ctx.Data["Title"] = fmt.Sprintf("#%d - %s", issue.Index, issue.Title)
	ctx.Data["Issue"] = issue

	if !issue.IsPull {
		ctx.NotFound("ViewPullCommits", nil)
		return nil
	}

	if err = issue.LoadPullRequest(); err != nil {
		ctx.ServerError("LoadPullRequest", err)
		return nil
	}

	if err = issue.PullRequest.GetHeadRepo(); err != nil {
		ctx.ServerError("GetHeadRepo", err)
		return nil
	}

	if ctx.IsSigned {
		// Update issue-user.
		if err = issue.ReadBy(ctx.User.ID); err != nil {
			ctx.ServerError("ReadBy", err)
			return nil
		}
	}

	return issue
}

func setMergeTarget(ctx *context.Context, pull *models.PullRequest) {
	if ctx.Repo.Owner.Name == pull.HeadUserName {
		ctx.Data["HeadTarget"] = pull.HeadBranch
	} else if pull.HeadRepo == nil {
		ctx.Data["HeadTarget"] = pull.HeadUserName + ":" + pull.HeadBranch
	} else {
		ctx.Data["HeadTarget"] = pull.HeadUserName + "/" + pull.HeadRepo.Name + ":" + pull.HeadBranch
	}
	ctx.Data["BaseTarget"] = pull.BaseBranch
}

// PrepareMergedViewPullInfo show meta information for a merged pull request view page
func PrepareMergedViewPullInfo(ctx *context.Context, issue *models.Issue) *git.PullRequestInfo {
	pull := issue.PullRequest

	setMergeTarget(ctx, pull)
	ctx.Data["HasMerged"] = true

	prInfo, err := ctx.Repo.GitRepo.GetPullRequestInfo(ctx.Repo.Repository.RepoPath(),
		pull.MergeBase, pull.GetGitRefName())

	if err != nil {
		if strings.Contains(err.Error(), "fatal: Not a valid object name") {
			ctx.Data["IsPullRequestBroken"] = true
			ctx.Data["BaseTarget"] = "deleted"
			ctx.Data["NumCommits"] = 0
			ctx.Data["NumFiles"] = 0
			return nil
		}

		ctx.ServerError("GetPullRequestInfo", err)
		return nil
	}
	ctx.Data["NumCommits"] = prInfo.Commits.Len()
	ctx.Data["NumFiles"] = prInfo.NumFiles
	return prInfo
}

// PrepareViewPullInfo show meta information for a pull request preview page
func PrepareViewPullInfo(ctx *context.Context, issue *models.Issue) *git.PullRequestInfo {
	repo := ctx.Repo.Repository
	pull := issue.PullRequest

	var err error
	if err = pull.GetHeadRepo(); err != nil {
		ctx.ServerError("GetHeadRepo", err)
		return nil
	}

	setMergeTarget(ctx, pull)

	var headGitRepo *git.Repository
	if pull.HeadRepo != nil {
		headGitRepo, err = git.OpenRepository(pull.HeadRepo.RepoPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return nil
		}
	}

	if pull.HeadRepo == nil || !headGitRepo.IsBranchExist(pull.HeadBranch) {
		ctx.Data["IsPullRequestBroken"] = true
		ctx.Data["HeadTarget"] = "deleted"
		ctx.Data["NumCommits"] = 0
		ctx.Data["NumFiles"] = 0
		return nil
	}

	prInfo, err := headGitRepo.GetPullRequestInfo(models.RepoPath(repo.Owner.Name, repo.Name),
		pull.BaseBranch, pull.HeadBranch)
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Not a valid object name") {
			ctx.Data["IsPullRequestBroken"] = true
			ctx.Data["BaseTarget"] = "deleted"
			ctx.Data["NumCommits"] = 0
			ctx.Data["NumFiles"] = 0
			return nil
		}

		ctx.ServerError("GetPullRequestInfo", err)
		return nil
	}

	if pull.IsWorkInProgress() {
		ctx.Data["IsPullWorkInProgress"] = true
		ctx.Data["WorkInProgressPrefix"] = pull.GetWorkInProgressPrefix()
	}

	if pull.IsFilesConflicted() {
		ctx.Data["IsPullFilesConflicted"] = true
		ctx.Data["ConflictedFiles"] = pull.ConflictedFiles
	}

	ctx.Data["NumCommits"] = prInfo.Commits.Len()
	ctx.Data["NumFiles"] = prInfo.NumFiles
	return prInfo
}

// ViewPullCommits show commits for a pull request
func ViewPullCommits(ctx *context.Context) {
	ctx.Data["PageIsPullList"] = true
	ctx.Data["PageIsPullCommits"] = true

	issue := checkPullInfo(ctx)
	if ctx.Written() {
		return
	}
	pull := issue.PullRequest

	var commits *list.List
	if pull.HasMerged {
		prInfo := PrepareMergedViewPullInfo(ctx, issue)
		if ctx.Written() {
			return
		} else if prInfo == nil {
			ctx.NotFound("ViewPullCommits", nil)
			return
		}
		ctx.Data["Username"] = ctx.Repo.Owner.Name
		ctx.Data["Reponame"] = ctx.Repo.Repository.Name
		commits = prInfo.Commits
	} else {
		prInfo := PrepareViewPullInfo(ctx, issue)
		if ctx.Written() {
			return
		} else if prInfo == nil {
			ctx.NotFound("ViewPullCommits", nil)
			return
		}
		ctx.Data["Username"] = pull.HeadUserName
		ctx.Data["Reponame"] = pull.HeadRepo.Name
		commits = prInfo.Commits
	}

	commits = models.ValidateCommitsWithEmails(commits)
	commits = models.ParseCommitsWithSignature(commits)
	commits = models.ParseCommitsWithStatus(commits, ctx.Repo.Repository)
	ctx.Data["Commits"] = commits
	ctx.Data["CommitCount"] = commits.Len()

	ctx.HTML(200, tplPullCommits)
}

// ViewPullFiles render pull request changed files list page
func ViewPullFiles(ctx *context.Context) {
	ctx.Data["PageIsPullList"] = true
	ctx.Data["PageIsPullFiles"] = true

	issue := checkPullInfo(ctx)
	if ctx.Written() {
		return
	}
	pull := issue.PullRequest

	whitespaceFlags := map[string]string{
		"ignore-all":    "-w",
		"ignore-change": "-b",
		"ignore-eol":    "--ignore-space-at-eol",
		"":              ""}

	var (
		diffRepoPath  string
		startCommitID string
		endCommitID   string
		gitRepo       *git.Repository
	)

	var headTarget string
	if pull.HasMerged {
		prInfo := PrepareMergedViewPullInfo(ctx, issue)
		if ctx.Written() {
			return
		} else if prInfo == nil {
			ctx.NotFound("ViewPullFiles", nil)
			return
		}

		diffRepoPath = ctx.Repo.GitRepo.Path
		gitRepo = ctx.Repo.GitRepo

		headCommitID, err := gitRepo.GetRefCommitID(pull.GetGitRefName())
		if err != nil {
			ctx.ServerError("GetRefCommitID", err)
			return
		}

		startCommitID = prInfo.MergeBase
		endCommitID = headCommitID

		headTarget = path.Join(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
		ctx.Data["Username"] = ctx.Repo.Owner.Name
		ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	} else {
		prInfo := PrepareViewPullInfo(ctx, issue)
		if ctx.Written() {
			return
		} else if prInfo == nil {
			ctx.NotFound("ViewPullFiles", nil)
			return
		}

		headRepoPath := models.RepoPath(pull.HeadUserName, pull.HeadRepo.Name)

		headGitRepo, err := git.OpenRepository(headRepoPath)
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return
		}

		headCommitID, err := headGitRepo.GetBranchCommitID(pull.HeadBranch)
		if err != nil {
			ctx.ServerError("GetBranchCommitID", err)
			return
		}

		diffRepoPath = headRepoPath
		startCommitID = prInfo.MergeBase
		endCommitID = headCommitID
		gitRepo = headGitRepo

		headTarget = path.Join(pull.HeadUserName, pull.HeadRepo.Name)
		ctx.Data["Username"] = pull.HeadUserName
		ctx.Data["Reponame"] = pull.HeadRepo.Name
	}

	diff, err := models.GetDiffRangeWithWhitespaceBehavior(diffRepoPath,
		startCommitID, endCommitID, setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles,
		whitespaceFlags[ctx.Data["WhitespaceBehavior"].(string)])
	if err != nil {
		ctx.ServerError("GetDiffRangeWithWhitespaceBehavior", err)
		return
	}

	if err = diff.LoadComments(issue, ctx.User); err != nil {
		ctx.ServerError("LoadComments", err)
		return
	}

	ctx.Data["Diff"] = diff
	ctx.Data["DiffNotAvailable"] = diff.NumFiles() == 0

	commit, err := gitRepo.GetCommit(endCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return
	}

	ctx.Data["IsImageFile"] = commit.IsImageFile
	ctx.Data["SourcePath"] = setting.AppSubURL + "/" + path.Join(headTarget, "src", "commit", endCommitID)
	ctx.Data["BeforeSourcePath"] = setting.AppSubURL + "/" + path.Join(headTarget, "src", "commit", startCommitID)
	ctx.Data["RawPath"] = setting.AppSubURL + "/" + path.Join(headTarget, "raw", "commit", endCommitID)
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireTribute"] = true
	if ctx.Data["Assignees"], err = ctx.Repo.Repository.GetAssignees(); err != nil {
		ctx.ServerError("GetAssignees", err)
		return
	}
	ctx.Data["CurrentReview"], err = models.GetCurrentReview(ctx.User, issue)
	if err != nil && !models.IsErrReviewNotExist(err) {
		ctx.ServerError("GetCurrentReview", err)
		return
	}
	ctx.HTML(200, tplPullFiles)
}

// MergePullRequest response for merging pull request
func MergePullRequest(ctx *context.Context, form auth.MergePullRequestForm) {
	issue := checkPullInfo(ctx)
	if ctx.Written() {
		return
	}
	if issue.IsClosed {
		ctx.NotFound("MergePullRequest", nil)
		return
	}

	pr := issue.PullRequest

	if !pr.CanAutoMerge() || pr.HasMerged {
		ctx.NotFound("MergePullRequest", nil)
		return
	}

	if pr.IsWorkInProgress() {
		ctx.Flash.Error(ctx.Tr("repo.pulls.no_merge_wip"))
		ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(pr.Index))
		return
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(pr.Index))
		return
	}

	message := strings.TrimSpace(form.MergeTitleField)
	if len(message) == 0 {
		if models.MergeStyle(form.Do) == models.MergeStyleMerge {
			message = pr.GetDefaultMergeMessage()
		}
		if models.MergeStyle(form.Do) == models.MergeStyleRebaseMerge {
			message = pr.GetDefaultMergeMessage()
		}
		if models.MergeStyle(form.Do) == models.MergeStyleSquash {
			message = pr.GetDefaultSquashMessage()
		}
	}

	form.MergeMessageField = strings.TrimSpace(form.MergeMessageField)
	if len(form.MergeMessageField) > 0 {
		message += "\n\n" + form.MergeMessageField
	}

	pr.Issue = issue
	pr.Issue.Repo = ctx.Repo.Repository

	noDeps, err := models.IssueNoDependenciesLeft(issue)
	if err != nil {
		return
	}

	if !noDeps {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.pr_close_blocked"))
		ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(pr.Index))
		return
	}

	if err = pr.Merge(ctx.User, ctx.Repo.GitRepo, models.MergeStyle(form.Do), message); err != nil {
		if models.IsErrInvalidMergeStyle(err) {
			ctx.Flash.Error(ctx.Tr("repo.pulls.invalid_merge_option"))
			ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(pr.Index))
			return
		}
		ctx.ServerError("Merge", err)
		return
	}

	if err := stopTimerIfAvailable(ctx.User, issue); err != nil {
		ctx.ServerError("CreateOrStopIssueStopwatch", err)
		return
	}

	notification.NotifyMergePullRequest(pr, ctx.User, ctx.Repo.GitRepo)

	log.Trace("Pull request merged: %d", pr.ID)
	ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(pr.Index))
}

func stopTimerIfAvailable(user *models.User, issue *models.Issue) error {

	if models.StopwatchExists(user.ID, issue.ID) {
		if err := models.CreateOrStopIssueStopwatch(user, issue); err != nil {
			return err
		}
	}

	return nil
}

// ParseCompareInfo parse compare info between two commit for preparing pull request
func ParseCompareInfo(ctx *context.Context) (*models.User, *models.Repository, *git.Repository, *git.PullRequestInfo, string, string) {
	baseRepo := ctx.Repo.Repository

	// Get compared branches information
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	var (
		headUser   *models.User
		headBranch string
		isSameRepo bool
		infoPath   string
		err        error
	)
	infoPath = ctx.Params("*")
	infos := strings.Split(infoPath, "...")
	if len(infos) != 2 {
		log.Trace("ParseCompareInfo[%d]: not enough compared branches information %s", baseRepo.ID, infos)
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil, nil, nil, nil, "", ""
	}

	baseBranch := infos[0]
	ctx.Data["BaseBranch"] = baseBranch

	// If there is no head repository, it means pull request between same repository.
	headInfos := strings.Split(infos[1], ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		headUser = ctx.Repo.Owner
		headBranch = headInfos[0]

	} else if len(headInfos) == 2 {
		headUser, err = models.GetUserByName(headInfos[0])
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.NotFound("GetUserByName", nil)
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return nil, nil, nil, nil, "", ""
		}
		headBranch = headInfos[1]
		isSameRepo = headUser.ID == ctx.Repo.Owner.ID
	} else {
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil, nil, nil, nil, "", ""
	}
	ctx.Data["HeadUser"] = headUser
	ctx.Data["HeadBranch"] = headBranch
	ctx.Repo.PullRequest.SameRepo = isSameRepo

	// Check if base branch is valid.
	if !ctx.Repo.GitRepo.IsBranchExist(baseBranch) {
		ctx.NotFound("IsBranchExist", nil)
		return nil, nil, nil, nil, "", ""
	}

	// Check if current user has fork of repository or in the same repository.
	headRepo, has := models.HasForkedRepo(headUser.ID, baseRepo.ID)
	if !has && !isSameRepo {
		log.Trace("ParseCompareInfo[%d]: does not have fork or in same repository", baseRepo.ID)
		ctx.NotFound("ParseCompareInfo", nil)
		return nil, nil, nil, nil, "", ""
	}

	var headGitRepo *git.Repository
	if isSameRepo {
		headRepo = ctx.Repo.Repository
		headGitRepo = ctx.Repo.GitRepo
		ctx.Data["BaseName"] = headUser.Name
	} else {
		headGitRepo, err = git.OpenRepository(models.RepoPath(headUser.Name, headRepo.Name))
		ctx.Data["BaseName"] = baseRepo.OwnerName
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return nil, nil, nil, nil, "", ""
		}
	}

	// user should have permission to read baseRepo's codes and pulls, NOT headRepo's
	permBase, err := models.GetUserRepoPermission(baseRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permBase.CanReadIssuesOrPulls(true) || !permBase.CanRead(models.UnitTypeCode) {
		log.Trace("Permission Denied: User: %s cannot create/read pull requests or cannot read code in Repo: %s\nUser in baseRepo has Permissions: %-+v",
			ctx.User.Name,
			baseRepo.RepoPath(),
			permBase)
		ctx.NotFound("GetUserRepoPermission", nil)
		return nil, nil, nil, nil, "", ""
	}

	// user should have permission to read headrepo's codes
	permHead, err := models.GetUserRepoPermission(headRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permHead.CanRead(models.UnitTypeCode) {
		log.Trace("Permission Denied: User: %-v cannot read code requests in Repo: %-v\nUser in headRepo has Permissions: %-+v",
			ctx.User,
			headRepo,
			permHead)
		ctx.NotFound("GetUserRepoPermission", nil)
		return nil, nil, nil, nil, "", ""
	}

	// Check if head branch is valid.
	if !headGitRepo.IsBranchExist(headBranch) {
		ctx.NotFound("IsBranchExist", nil)
		return nil, nil, nil, nil, "", ""
	}

	headBranches, err := headGitRepo.GetBranches()
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return nil, nil, nil, nil, "", ""
	}
	ctx.Data["HeadBranches"] = headBranches

	prInfo, err := headGitRepo.GetPullRequestInfo(models.RepoPath(baseRepo.Owner.Name, baseRepo.Name), baseBranch, headBranch)
	if err != nil {
		ctx.ServerError("GetPullRequestInfo", err)
		return nil, nil, nil, nil, "", ""
	}
	ctx.Data["BeforeCommitID"] = prInfo.MergeBase

	return headUser, headRepo, headGitRepo, prInfo, baseBranch, headBranch
}

// PrepareCompareDiff render pull request preview diff page
func PrepareCompareDiff(
	ctx *context.Context,
	headUser *models.User,
	headRepo *models.Repository,
	headGitRepo *git.Repository,
	prInfo *git.PullRequestInfo,
	baseBranch, headBranch string) bool {

	var (
		repo  = ctx.Repo.Repository
		err   error
		title string
	)

	// Get diff information.
	ctx.Data["CommitRepoLink"] = headRepo.Link()

	headCommitID, err := headGitRepo.GetBranchCommitID(headBranch)
	if err != nil {
		ctx.ServerError("GetBranchCommitID", err)
		return false
	}
	ctx.Data["AfterCommitID"] = headCommitID

	if headCommitID == prInfo.MergeBase {
		ctx.Data["IsNothingToCompare"] = true
		return true
	}

	diff, err := models.GetDiffRange(models.RepoPath(headUser.Name, headRepo.Name),
		prInfo.MergeBase, headCommitID, setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles)
	if err != nil {
		ctx.ServerError("GetDiffRange", err)
		return false
	}
	ctx.Data["Diff"] = diff
	ctx.Data["DiffNotAvailable"] = diff.NumFiles() == 0

	headCommit, err := headGitRepo.GetCommit(headCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return false
	}

	prInfo.Commits = models.ValidateCommitsWithEmails(prInfo.Commits)
	prInfo.Commits = models.ParseCommitsWithSignature(prInfo.Commits)
	prInfo.Commits = models.ParseCommitsWithStatus(prInfo.Commits, headRepo)
	ctx.Data["Commits"] = prInfo.Commits
	ctx.Data["CommitCount"] = prInfo.Commits.Len()

	if prInfo.Commits.Len() == 1 {
		c := prInfo.Commits.Front().Value.(models.SignCommitWithStatuses)
		title = strings.TrimSpace(c.UserCommit.Summary())

		body := strings.Split(strings.TrimSpace(c.UserCommit.Message()), "\n")
		if len(body) > 1 {
			ctx.Data["content"] = strings.Join(body[1:], "\n")
		}
	} else {
		title = headBranch
	}

	ctx.Data["title"] = title
	ctx.Data["Username"] = headUser.Name
	ctx.Data["Reponame"] = headRepo.Name
	ctx.Data["IsImageFile"] = headCommit.IsImageFile

	headTarget := path.Join(headUser.Name, repo.Name)
	ctx.Data["SourcePath"] = setting.AppSubURL + "/" + path.Join(headTarget, "src", "commit", headCommitID)
	ctx.Data["BeforeSourcePath"] = setting.AppSubURL + "/" + path.Join(headTarget, "src", "commit", prInfo.MergeBase)
	ctx.Data["RawPath"] = setting.AppSubURL + "/" + path.Join(headTarget, "raw", "commit", headCommitID)
	return false
}

// CompareAndPullRequest render pull request preview page
func CompareAndPullRequest(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.pulls.compare_changes")
	ctx.Data["PageIsComparePull"] = true
	ctx.Data["IsDiffCompare"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	setTemplateIfExists(ctx, pullRequestTemplateKey, pullRequestTemplateCandidates)
	renderAttachmentSettings(ctx)

	headUser, headRepo, headGitRepo, prInfo, baseBranch, headBranch := ParseCompareInfo(ctx)
	if ctx.Written() {
		return
	}

	pr, err := models.GetUnmergedPullRequest(headRepo.ID, ctx.Repo.Repository.ID, headBranch, baseBranch)
	if err != nil {
		if !models.IsErrPullRequestNotExist(err) {
			ctx.ServerError("GetUnmergedPullRequest", err)
			return
		}
	} else {
		ctx.Data["HasPullRequest"] = true
		ctx.Data["PullRequest"] = pr
		ctx.HTML(200, tplComparePull)
		return
	}

	nothingToCompare := PrepareCompareDiff(ctx, headUser, headRepo, headGitRepo, prInfo, baseBranch, headBranch)
	if ctx.Written() {
		return
	}

	if !nothingToCompare {
		// Setup information for new form.
		RetrieveRepoMetas(ctx, ctx.Repo.Repository)
		if ctx.Written() {
			return
		}
	}

	ctx.HTML(200, tplComparePull)
}

// CompareAndPullRequestPost response for creating pull request
func CompareAndPullRequestPost(ctx *context.Context, form auth.CreateIssueForm) {
	ctx.Data["Title"] = ctx.Tr("repo.pulls.compare_changes")
	ctx.Data["PageIsComparePull"] = true
	ctx.Data["IsDiffCompare"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	renderAttachmentSettings(ctx)

	var (
		repo        = ctx.Repo.Repository
		attachments []string
	)

	headUser, headRepo, headGitRepo, prInfo, baseBranch, headBranch := ParseCompareInfo(ctx)
	if ctx.Written() {
		return
	}

	labelIDs, assigneeIDs, milestoneID := ValidateRepoMetas(ctx, form, true)
	if ctx.Written() {
		return
	}

	if setting.AttachmentEnabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		auth.AssignForm(form, ctx.Data)

		// This stage is already stop creating new pull request, so it does not matter if it has
		// something to compare or not.
		PrepareCompareDiff(ctx, headUser, headRepo, headGitRepo, prInfo, baseBranch, headBranch)
		if ctx.Written() {
			return
		}

		ctx.HTML(200, tplComparePull)
		return
	}

	if util.IsEmptyString(form.Title) {
		PrepareCompareDiff(ctx, headUser, headRepo, headGitRepo, prInfo, baseBranch, headBranch)
		if ctx.Written() {
			return
		}

		ctx.RenderWithErr(ctx.Tr("repo.issues.new.title_empty"), tplComparePull, form)
		return
	}

	patch, err := headGitRepo.GetPatch(prInfo.MergeBase, headBranch)
	if err != nil {
		ctx.ServerError("GetPatch", err)
		return
	}

	pullIssue := &models.Issue{
		RepoID:      repo.ID,
		Index:       repo.NextIssueIndex(),
		Title:       form.Title,
		PosterID:    ctx.User.ID,
		Poster:      ctx.User,
		MilestoneID: milestoneID,
		IsPull:      true,
		Content:     form.Content,
	}
	pullRequest := &models.PullRequest{
		HeadRepoID:   headRepo.ID,
		BaseRepoID:   repo.ID,
		HeadUserName: headUser.Name,
		HeadBranch:   headBranch,
		BaseBranch:   baseBranch,
		HeadRepo:     headRepo,
		BaseRepo:     repo,
		MergeBase:    prInfo.MergeBase,
		Type:         models.PullRequestGitea,
	}
	// FIXME: check error in the case two people send pull request at almost same time, give nice error prompt
	// instead of 500.

	if err := models.NewPullRequest(repo, pullIssue, labelIDs, attachments, pullRequest, patch, assigneeIDs); err != nil {
		if models.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(400, "UserDoesNotHaveAccessToRepo", err.Error())
			return
		}
		ctx.ServerError("NewPullRequest", err)
		return
	} else if err := pullRequest.PushToBaseRepo(); err != nil {
		ctx.ServerError("PushToBaseRepo", err)
		return
	}

	notification.NotifyNewPullRequest(pullRequest)

	log.Trace("Pull request created: %d/%d", repo.ID, pullIssue.ID)
	ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(pullIssue.Index))
}

// TriggerTask response for a trigger task request
func TriggerTask(ctx *context.Context) {
	pusherID := ctx.QueryInt64("pusher")
	branch := ctx.Query("branch")
	secret := ctx.Query("secret")
	if len(branch) == 0 || len(secret) == 0 || pusherID <= 0 {
		ctx.Error(404)
		log.Trace("TriggerTask: branch or secret is empty, or pusher ID is not valid")
		return
	}
	owner, repo := parseOwnerAndRepo(ctx)
	if ctx.Written() {
		return
	}
	if secret != base.EncodeMD5(owner.Salt) {
		ctx.Error(404)
		log.Trace("TriggerTask [%s/%s]: invalid secret", owner.Name, repo.Name)
		return
	}

	pusher, err := models.GetUserByID(pusherID)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(404)
		} else {
			ctx.ServerError("GetUserByID", err)
		}
		return
	}

	log.Trace("TriggerTask '%s/%s' by %s", repo.Name, branch, pusher.Name)

	go models.HookQueue.Add(repo.ID)
	go models.AddTestPullRequestTask(pusher, repo.ID, branch, true)
	ctx.Status(202)
}

// CleanUpPullRequest responses for delete merged branch when PR has been merged
func CleanUpPullRequest(ctx *context.Context) {
	issue := checkPullInfo(ctx)
	if ctx.Written() {
		return
	}

	pr := issue.PullRequest

	// Allow cleanup only for merged PR
	if !pr.HasMerged {
		ctx.NotFound("CleanUpPullRequest", nil)
		return
	}

	if err := pr.GetHeadRepo(); err != nil {
		ctx.ServerError("GetHeadRepo", err)
		return
	} else if pr.HeadRepo == nil {
		// Forked repository has already been deleted
		ctx.NotFound("CleanUpPullRequest", nil)
		return
	} else if pr.GetBaseRepo(); err != nil {
		ctx.ServerError("GetBaseRepo", err)
		return
	} else if pr.HeadRepo.GetOwner(); err != nil {
		ctx.ServerError("HeadRepo.GetOwner", err)
		return
	}

	perm, err := models.GetUserRepoPermission(pr.HeadRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return
	}
	if !perm.CanWrite(models.UnitTypeCode) {
		ctx.NotFound("CleanUpPullRequest", nil)
		return
	}

	fullBranchName := pr.HeadRepo.Owner.Name + "/" + pr.HeadBranch

	gitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
	if err != nil {
		ctx.ServerError(fmt.Sprintf("OpenRepository[%s]", pr.HeadRepo.RepoPath()), err)
		return
	}

	gitBaseRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		ctx.ServerError(fmt.Sprintf("OpenRepository[%s]", pr.BaseRepo.RepoPath()), err)
		return
	}

	defer func() {
		ctx.JSON(200, map[string]interface{}{
			"redirect": pr.BaseRepo.Link() + "/pulls/" + com.ToStr(issue.Index),
		})
	}()

	if pr.HeadBranch == pr.HeadRepo.DefaultBranch || !gitRepo.IsBranchExist(pr.HeadBranch) {
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}

	// Check if branch is not protected
	if protected, err := pr.HeadRepo.IsProtectedBranch(pr.HeadBranch, ctx.User); err != nil || protected {
		if err != nil {
			log.Error(4, "HeadRepo.IsProtectedBranch: %v", err)
		}
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}

	// Check if branch has no new commits
	headCommitID, err := gitBaseRepo.GetRefCommitID(pr.GetGitRefName())
	if err != nil {
		log.Error(4, "GetRefCommitID: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}
	branchCommitID, err := gitRepo.GetBranchCommitID(pr.HeadBranch)
	if err != nil {
		log.Error(4, "GetBranchCommitID: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}
	if headCommitID != branchCommitID {
		ctx.Flash.Error(ctx.Tr("repo.branch.delete_branch_has_new_commits", fullBranchName))
		return
	}

	if err := gitRepo.DeleteBranch(pr.HeadBranch, git.DeleteBranchOptions{
		Force: true,
	}); err != nil {
		log.Error(4, "DeleteBranch: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}

	if err := models.AddDeletePRBranchComment(ctx.User, pr.BaseRepo, issue.ID, pr.HeadBranch); err != nil {
		// Do not fail here as branch has already been deleted
		log.Error(4, "DeleteBranch: %v", err)
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.deletion_success", fullBranchName))
}

// DownloadPullDiff render a pull's raw diff
func DownloadPullDiff(ctx *context.Context) {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return
	}

	// Return not found if it's not a pull request
	if !issue.IsPull {
		ctx.NotFound("DownloadPullDiff",
			fmt.Errorf("Issue is not a pull request"))
		return
	}

	if err = issue.LoadPullRequest(); err != nil {
		ctx.ServerError("LoadPullRequest", err)
		return
	}

	pr := issue.PullRequest
	if err = pr.GetBaseRepo(); err != nil {
		ctx.ServerError("GetBaseRepo", err)
		return
	}
	patch, err := pr.BaseRepo.PatchPath(pr.Index)
	if err != nil {
		ctx.ServerError("PatchPath", err)
		return
	}

	ctx.ServeFileContent(patch)
}

// DownloadPullPatch render a pull's raw patch
func DownloadPullPatch(ctx *context.Context) {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return
	}

	// Return not found if it's not a pull request
	if !issue.IsPull {
		ctx.NotFound("DownloadPullDiff",
			fmt.Errorf("Issue is not a pull request"))
		return
	}

	if err = issue.LoadPullRequest(); err != nil {
		ctx.ServerError("LoadPullRequest", err)
		return
	}

	pr := issue.PullRequest
	if err = pr.GetHeadRepo(); err != nil {
		ctx.ServerError("GetHeadRepo", err)
		return
	}

	headGitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return
	}

	patch, err := headGitRepo.GetFormatPatch(pr.MergeBase, pr.HeadBranch)
	if err != nil {
		ctx.ServerError("GetFormatPatch", err)
		return
	}

	_, err = io.Copy(ctx, patch)
	if err != nil {
		ctx.ServerError("io.Copy", err)
		return
	}
}
