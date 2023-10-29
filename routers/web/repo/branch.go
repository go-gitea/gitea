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
)

const (
	tplBranch base.TplName = "repo/branch/list"
)

// Branches render repository branch page
func Branches(ctx *context.Context) {
	ctx.Data["Title"] = "Branches"
	ctx.Data["IsRepoToolbarBranches"] = true
	ctx.Data["AllowsPulls"] = ctx.Repo.Repository.AllowsPulls(ctx)
	ctx.Data["IsWriter"] = ctx.Repo.CanWrite(unit.TypeCode)
	ctx.Data["IsMirror"] = ctx.Repo.Repository.IsMirror
	ctx.Data["CanPull"] = ctx.Repo.CanWrite(unit.TypeCode) ||
		(ctx.IsSigned && repo_model.HasForkedRepo(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID))
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["PageIsBranches"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	pageSize := setting.Git.BranchesRangeSize

	kw := ctx.FormString("q")

	defaultBranch, branches, branchesCount, err := repo_service.LoadBranches(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, util.OptionalBoolNone, kw, page, pageSize)
	if err != nil {
		ctx.ServerError("LoadBranches", err)
		return
	}

	commitIDs := []string{defaultBranch.DBBranch.CommitID}
	for _, branch := range branches {
		commitIDs = append(commitIDs, branch.DBBranch.CommitID)
	}

	commitStatuses, err := git_model.GetLatestCommitStatusForRepoCommitIDs(ctx, ctx.Repo.Repository.ID, commitIDs)
	if err != nil {
		ctx.ServerError("LoadBranches", err)
		return
	}

	commitStatus := make(map[string]*git_model.CommitStatus)
	for commitID, cs := range commitStatuses {
		commitStatus[commitID] = git_model.CalcCommitStatus(cs)
	}

	ctx.Data["Keyword"] = kw
	ctx.Data["Branches"] = branches
	ctx.Data["CommitStatus"] = commitStatus
	ctx.Data["CommitStatuses"] = commitStatuses
	ctx.Data["DefaultBranchBranch"] = defaultBranch
	pager := context.NewPagination(int(branchesCount), pageSize, page, 5)
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
		Branch: fmt.Sprintf("%s:%s%s", deletedBranch.CommitID, git.BranchPrefix, deletedBranch.Name),
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
			NewCommitID:  deletedBranch.CommitID,
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
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/branches?page=" + url.QueryEscape(ctx.FormString("page")))
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
		if git_model.IsErrBranchAlreadyExists(err) || git.IsErrPushOutOfDate(err) {
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_already_exists", form.NewBranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if git_model.IsErrBranchNameConflict(err) {
			e := err.(git_model.ErrBranchNameConflict)
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
