// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/forms"
	release_service "code.gitea.io/gitea/services/release"
	repo_service "code.gitea.io/gitea/services/repository"
	files_service "code.gitea.io/gitea/services/repository/files"
)

const (
	tplBranch base.TplName = "repo/branch/list"
)

// Branch contains the branch information
type Branch struct {
	Name              string
	Commit            *git.Commit
	IsProtected       bool
	IsDeleted         bool
	IsIncluded        bool
	DeletedBranch     *git_model.DeletedBranch
	CommitsAhead      int
	CommitsBehind     int
	LatestPullRequest *issues_model.PullRequest
	MergeMovedOn      bool
}

// Branches render repository branch page
func Branches(ctx *context.Context) {
	ctx.Data["Title"] = "Branches"
	ctx.Data["IsRepoToolbarBranches"] = true
	ctx.Data["DefaultBranch"] = ctx.Repo.Repository.DefaultBranch
	ctx.Data["AllowsPulls"] = ctx.Repo.Repository.AllowsPulls()
	ctx.Data["IsWriter"] = ctx.Repo.CanWrite(unit.TypeCode)
	ctx.Data["IsMirror"] = ctx.Repo.Repository.IsMirror
	ctx.Data["CanPull"] = ctx.Repo.CanWrite(unit.TypeCode) ||
		(ctx.IsSigned && repo_model.HasForkedRepo(ctx.Doer.ID, ctx.Repo.Repository.ID))
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["PageIsBranches"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	pageSize := setting.Git.BranchesRangeSize

	skip := (page - 1) * pageSize
	log.Debug("Branches: skip: %d limit: %d", skip, pageSize)
	defaultBranchBranch, branches, branchesCount := loadBranches(ctx, skip, pageSize)
	if ctx.Written() {
		return
	}
	ctx.Data["Branches"] = branches
	ctx.Data["DefaultBranchBranch"] = defaultBranchBranch
	pager := context.NewPagination(branchesCount, pageSize, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplBranch)
}

// DeleteBranchPost responses for delete merged branch
func DeleteBranchPost(ctx *context.Context) {
	defer redirect(ctx)
	branchName := ctx.FormString("name")

	if err := repo_service.DeleteBranch(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, branchName); err != nil {
		switch {
		case git.IsErrBranchNotExist(err):
			log.Debug("DeleteBranch: Can't delete non existing branch '%s'", branchName)
			ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", branchName))
		case errors.Is(err, repo_service.ErrBranchIsDefault):
			log.Debug("DeleteBranch: Can't delete default branch '%s'", branchName)
			ctx.Flash.Error(ctx.Tr("repo.branch.default_deletion_failed", branchName))
		case errors.Is(err, git_model.ErrBranchIsProtected):
			log.Debug("DeleteBranch: Can't delete protected branch '%s'", branchName)
			ctx.Flash.Error(ctx.Tr("repo.branch.protected_deletion_failed", branchName))
		default:
			log.Error("DeleteBranch: %v", err)
			ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", branchName))
		}

		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.deletion_success", branchName))
}

// RestoreBranchPost responses for delete merged branch
func RestoreBranchPost(ctx *context.Context) {
	defer redirect(ctx)

	branchID := ctx.FormInt64("branch_id")
	branchName := ctx.FormString("name")

	deletedBranch, err := git_model.GetDeletedBranchByID(ctx, ctx.Repo.Repository.ID, branchID)
	if err != nil {
		log.Error("GetDeletedBranchByID: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.restore_failed", branchName))
		return
	} else if deletedBranch == nil {
		log.Debug("RestoreBranch: Can't restore branch[%d] '%s', as it does not exist", branchID, branchName)
		ctx.Flash.Error(ctx.Tr("repo.branch.restore_failed", branchName))
		return
	}

	if err := git.Push(ctx, ctx.Repo.Repository.RepoPath(), git.PushOptions{
		Remote: ctx.Repo.Repository.RepoPath(),
		Branch: fmt.Sprintf("%s:%s%s", deletedBranch.Commit, git.BranchPrefix, deletedBranch.Name),
		Env:    repo_module.PushingEnvironment(ctx.Doer, ctx.Repo.Repository),
	}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Debug("RestoreBranch: Can't restore branch '%s', since one with same name already exist", deletedBranch.Name)
			ctx.Flash.Error(ctx.Tr("repo.branch.already_exists", deletedBranch.Name))
			return
		}
		log.Error("RestoreBranch: CreateBranch: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.restore_failed", deletedBranch.Name))
		return
	}

	// Don't return error below this
	if err := repo_service.PushUpdate(
		&repo_module.PushUpdateOptions{
			RefFullName:  git.RefNameFromBranch(deletedBranch.Name),
			OldCommitID:  git.EmptySHA,
			NewCommitID:  deletedBranch.Commit,
			PusherID:     ctx.Doer.ID,
			PusherName:   ctx.Doer.Name,
			RepoUserName: ctx.Repo.Owner.Name,
			RepoName:     ctx.Repo.Repository.Name,
		}); err != nil {
		log.Error("RestoreBranch: Update: %v", err)
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.restore_success", deletedBranch.Name))
}

func redirect(ctx *context.Context) {
	ctx.JSON(http.StatusOK, map[string]any{
		"redirect": ctx.Repo.RepoLink + "/branches?page=" + url.QueryEscape(ctx.FormString("page")),
	})
}

// loadBranches loads branches from the repository limited by page & pageSize.
// NOTE: May write to context on error.
func loadBranches(ctx *context.Context, skip, limit int) (*Branch, []*Branch, int) {
	defaultBranch, err := ctx.Repo.GitRepo.GetBranch(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		if !git.IsErrBranchNotExist(err) {
			log.Error("loadBranches: get default branch: %v", err)
			ctx.ServerError("GetDefaultBranch", err)
			return nil, nil, 0
		}
		log.Warn("loadBranches: missing default branch %s for %-v", ctx.Repo.Repository.DefaultBranch, ctx.Repo.Repository)
	}

	rawBranches, totalNumOfBranches, err := ctx.Repo.GitRepo.GetBranches(skip, limit)
	if err != nil {
		log.Error("GetBranches: %v", err)
		ctx.ServerError("GetBranches", err)
		return nil, nil, 0
	}

	rules, err := git_model.FindRepoProtectedBranchRules(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("FindRepoProtectedBranchRules", err)
		return nil, nil, 0
	}

	repoIDToRepo := map[int64]*repo_model.Repository{}
	repoIDToRepo[ctx.Repo.Repository.ID] = ctx.Repo.Repository

	repoIDToGitRepo := map[int64]*git.Repository{}
	repoIDToGitRepo[ctx.Repo.Repository.ID] = ctx.Repo.GitRepo

	var branches []*Branch
	for i := range rawBranches {
		if defaultBranch != nil && rawBranches[i].Name == defaultBranch.Name {
			// Skip default branch
			continue
		}

		branch := loadOneBranch(ctx, rawBranches[i], defaultBranch, &rules, repoIDToRepo, repoIDToGitRepo)
		if branch == nil {
			return nil, nil, 0
		}

		branches = append(branches, branch)
	}

	var defaultBranchBranch *Branch
	if defaultBranch != nil {
		// Always add the default branch
		log.Debug("loadOneBranch: load default: '%s'", defaultBranch.Name)
		defaultBranchBranch = loadOneBranch(ctx, defaultBranch, defaultBranch, &rules, repoIDToRepo, repoIDToGitRepo)
		branches = append(branches, defaultBranchBranch)
	}

	if ctx.Repo.CanWrite(unit.TypeCode) {
		deletedBranches, err := getDeletedBranches(ctx)
		if err != nil {
			ctx.ServerError("getDeletedBranches", err)
			return nil, nil, 0
		}
		branches = append(branches, deletedBranches...)
	}

	return defaultBranchBranch, branches, totalNumOfBranches
}

func loadOneBranch(ctx *context.Context, rawBranch, defaultBranch *git.Branch, protectedBranches *git_model.ProtectedBranchRules,
	repoIDToRepo map[int64]*repo_model.Repository,
	repoIDToGitRepo map[int64]*git.Repository,
) *Branch {
	log.Trace("loadOneBranch: '%s'", rawBranch.Name)

	commit, err := rawBranch.GetCommit()
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return nil
	}

	branchName := rawBranch.Name
	p := protectedBranches.GetFirstMatched(branchName)
	isProtected := p != nil

	divergence := &git.DivergeObject{
		Ahead:  -1,
		Behind: -1,
	}
	if defaultBranch != nil {
		divergence, err = files_service.CountDivergingCommits(ctx, ctx.Repo.Repository, git.BranchPrefix+branchName)
		if err != nil {
			log.Error("CountDivergingCommits", err)
		}
	}

	pr, err := issues_model.GetLatestPullRequestByHeadInfo(ctx.Repo.Repository.ID, branchName)
	if err != nil {
		ctx.ServerError("GetLatestPullRequestByHeadInfo", err)
		return nil
	}
	headCommit := commit.ID.String()

	mergeMovedOn := false
	if pr != nil {
		pr.HeadRepo = ctx.Repo.Repository
		if err := pr.LoadIssue(ctx); err != nil {
			ctx.ServerError("LoadIssue", err)
			return nil
		}
		if repo, ok := repoIDToRepo[pr.BaseRepoID]; ok {
			pr.BaseRepo = repo
		} else if err := pr.LoadBaseRepo(ctx); err != nil {
			ctx.ServerError("LoadBaseRepo", err)
			return nil
		} else {
			repoIDToRepo[pr.BaseRepoID] = pr.BaseRepo
		}
		pr.Issue.Repo = pr.BaseRepo

		if pr.HasMerged {
			baseGitRepo, ok := repoIDToGitRepo[pr.BaseRepoID]
			if !ok {
				baseGitRepo, err = git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
				if err != nil {
					ctx.ServerError("OpenRepository", err)
					return nil
				}
				defer baseGitRepo.Close()
				repoIDToGitRepo[pr.BaseRepoID] = baseGitRepo
			}
			pullCommit, err := baseGitRepo.GetRefCommitID(pr.GetGitRefName())
			if err != nil && !git.IsErrNotExist(err) {
				ctx.ServerError("GetBranchCommitID", err)
				return nil
			}
			if err == nil && headCommit != pullCommit {
				// the head has moved on from the merge - we shouldn't delete
				mergeMovedOn = true
			}
		}
	}

	isIncluded := divergence.Ahead == 0 && ctx.Repo.Repository.DefaultBranch != branchName
	return &Branch{
		Name:              branchName,
		Commit:            commit,
		IsProtected:       isProtected,
		IsIncluded:        isIncluded,
		CommitsAhead:      divergence.Ahead,
		CommitsBehind:     divergence.Behind,
		LatestPullRequest: pr,
		MergeMovedOn:      mergeMovedOn,
	}
}

func getDeletedBranches(ctx *context.Context) ([]*Branch, error) {
	branches := []*Branch{}

	deletedBranches, err := git_model.GetDeletedBranches(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		return branches, err
	}

	for i := range deletedBranches {
		deletedBranches[i].LoadUser(ctx)
		branches = append(branches, &Branch{
			Name:          deletedBranches[i].Name,
			IsDeleted:     true,
			DeletedBranch: deletedBranches[i],
		})
	}

	return branches, nil
}

// CreateBranch creates new branch in repository
func CreateBranch(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewBranchForm)
	if !ctx.Repo.CanCreateBranch() {
		ctx.NotFound("CreateBranch", nil)
		return
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
		return
	}

	var err error

	if form.CreateTag {
		target := ctx.Repo.CommitID
		if ctx.Repo.IsViewBranch {
			target = ctx.Repo.BranchName
		}
		err = release_service.CreateNewTag(ctx, ctx.Doer, ctx.Repo.Repository, target, form.NewBranchName, "")
	} else if ctx.Repo.IsViewBranch {
		err = repo_service.CreateNewBranch(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.BranchName, form.NewBranchName)
	} else {
		err = repo_service.CreateNewBranchFromCommit(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.CommitID, form.NewBranchName)
	}
	if err != nil {
		if models.IsErrProtectedTagName(err) {
			ctx.Flash.Error(ctx.Tr("repo.release.tag_name_protected"))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}

		if models.IsErrTagAlreadyExists(err) {
			e := err.(models.ErrTagAlreadyExists)
			ctx.Flash.Error(ctx.Tr("repo.branch.tag_collision", e.TagName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if models.IsErrBranchAlreadyExists(err) || git.IsErrPushOutOfDate(err) {
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_already_exists", form.NewBranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if models.IsErrBranchNameConflict(err) {
			e := err.(models.ErrBranchNameConflict)
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_name_conflict", form.NewBranchName, e.BranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if git.IsErrPushRejected(err) {
			e := err.(*git.ErrPushRejected)
			if len(e.Message) == 0 {
				ctx.Flash.Error(ctx.Tr("repo.editor.push_rejected_no_message"))
			} else {
				flashError, err := ctx.RenderToString(tplAlertDetails, map[string]any{
					"Message": ctx.Tr("repo.editor.push_rejected"),
					"Summary": ctx.Tr("repo.editor.push_rejected_summary"),
					"Details": utils.SanitizeFlashErrorString(e.Message),
				})
				if err != nil {
					ctx.ServerError("UpdatePullRequest.HTMLString", err)
					return
				}
				ctx.Flash.Error(flashError)
			}
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}

		ctx.ServerError("CreateNewBranch", err)
		return
	}

	if form.CreateTag {
		ctx.Flash.Success(ctx.Tr("repo.tag.create_success", form.NewBranchName))
		ctx.Redirect(ctx.Repo.RepoLink + "/src/tag/" + util.PathEscapeSegments(form.NewBranchName))
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.create_success", form.NewBranchName))
	ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(form.NewBranchName) + "/" + util.PathEscapeSegments(form.CurrentPath))
}
