// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplBranch base.TplName = "repo/branch/list"
)

// Branch contains the branch information
type Branch struct {
	Name          string
	Commit        *git.Commit
	IsProtected   bool
	IsDeleted     bool
	DeletedBranch *models.DeletedBranch
}

// Branches render repository branch page
func Branches(ctx *context.Context) {
	ctx.Data["Title"] = "Branches"
	ctx.Data["IsRepoToolbarBranches"] = true
	ctx.Data["DefaultBranch"] = ctx.Repo.Repository.DefaultBranch
	ctx.Data["IsWriter"] = ctx.Repo.CanWrite(models.UnitTypeCode)
	ctx.Data["IsMirror"] = ctx.Repo.Repository.IsMirror
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["PageIsBranches"] = true

	ctx.Data["Branches"] = loadBranches(ctx)
	ctx.HTML(200, tplBranch)
}

// DeleteBranchPost responses for delete merged branch
func DeleteBranchPost(ctx *context.Context) {
	defer redirect(ctx)

	branchName := ctx.Query("name")
	isProtected, err := ctx.Repo.Repository.IsProtectedBranch(branchName, ctx.User)
	if err != nil {
		log.Error(4, "DeleteBranch: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", branchName))
		return
	}

	if isProtected {
		ctx.Flash.Error(ctx.Tr("repo.branch.protected_deletion_failed", branchName))
		return
	}

	if !ctx.Repo.GitRepo.IsBranchExist(branchName) || branchName == ctx.Repo.Repository.DefaultBranch {
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", branchName))
		return
	}

	if err := deleteBranch(ctx, branchName); err != nil {
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", branchName))
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.deletion_success", branchName))
}

// RestoreBranchPost responses for delete merged branch
func RestoreBranchPost(ctx *context.Context) {
	defer redirect(ctx)

	branchID := ctx.QueryInt64("branch_id")
	branchName := ctx.Query("name")

	deletedBranch, err := ctx.Repo.Repository.GetDeletedBranchByID(branchID)
	if err != nil {
		log.Error(4, "GetDeletedBranchByID: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.restore_failed", branchName))
		return
	}

	if err := ctx.Repo.GitRepo.CreateBranch(deletedBranch.Name, deletedBranch.Commit); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			ctx.Flash.Error(ctx.Tr("repo.branch.already_exists", deletedBranch.Name))
			return
		}
		log.Error(4, "CreateBranch: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.restore_failed", deletedBranch.Name))
		return
	}

	if err := ctx.Repo.Repository.RemoveDeletedBranch(deletedBranch.ID); err != nil {
		log.Error(4, "RemoveDeletedBranch: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.restore_failed", deletedBranch.Name))
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.restore_success", deletedBranch.Name))
}

func redirect(ctx *context.Context) {
	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/branches",
	})
}

func deleteBranch(ctx *context.Context, branchName string) error {
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(branchName)
	if err != nil {
		log.Error(4, "GetBranchCommit: %v", err)
		return err
	}

	if err := ctx.Repo.GitRepo.DeleteBranch(branchName, git.DeleteBranchOptions{
		Force: true,
	}); err != nil {
		log.Error(4, "DeleteBranch: %v", err)
		return err
	}

	// Don't return error below this
	if err := models.PushUpdate(branchName, models.PushUpdateOptions{
		RefFullName:  git.BranchPrefix + branchName,
		OldCommitID:  commit.ID.String(),
		NewCommitID:  git.EmptySHA,
		PusherID:     ctx.User.ID,
		PusherName:   ctx.User.Name,
		RepoUserName: ctx.Repo.Owner.Name,
		RepoName:     ctx.Repo.Repository.Name,
	}); err != nil {
		log.Error(4, "Update: %v", err)
	}

	if err := ctx.Repo.Repository.AddDeletedBranch(branchName, commit.ID.String(), ctx.User.ID); err != nil {
		log.Warn("AddDeletedBranch: %v", err)
	}

	return nil
}

func loadBranches(ctx *context.Context) []*Branch {
	rawBranches, err := ctx.Repo.Repository.GetBranches()
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return nil
	}

	branches := make([]*Branch, len(rawBranches))
	for i := range rawBranches {
		commit, err := rawBranches[i].GetCommit()
		if err != nil {
			ctx.ServerError("GetCommit", err)
			return nil
		}

		isProtected, err := ctx.Repo.Repository.IsProtectedBranch(rawBranches[i].Name, ctx.User)
		if err != nil {
			ctx.ServerError("IsProtectedBranch", err)
			return nil
		}

		branches[i] = &Branch{
			Name:        rawBranches[i].Name,
			Commit:      commit,
			IsProtected: isProtected,
		}
	}

	if ctx.Repo.CanWrite(models.UnitTypeCode) {
		deletedBranches, err := getDeletedBranches(ctx)
		if err != nil {
			ctx.ServerError("getDeletedBranches", err)
			return nil
		}
		branches = append(branches, deletedBranches...)
	}

	return branches
}

func getDeletedBranches(ctx *context.Context) ([]*Branch, error) {
	branches := []*Branch{}

	deletedBranches, err := ctx.Repo.Repository.GetDeletedBranches()
	if err != nil {
		return branches, err
	}

	for i := range deletedBranches {
		deletedBranches[i].LoadUser()
		branches = append(branches, &Branch{
			Name:          deletedBranches[i].Name,
			IsDeleted:     true,
			DeletedBranch: deletedBranches[i],
		})
	}

	return branches, nil
}

// CreateBranch creates new branch in repository
func CreateBranch(ctx *context.Context, form auth.NewBranchForm) {
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
	if ctx.Repo.IsViewBranch {
		err = ctx.Repo.Repository.CreateNewBranch(ctx.User, ctx.Repo.BranchName, form.NewBranchName)
	} else {
		err = ctx.Repo.Repository.CreateNewBranchFromCommit(ctx.User, ctx.Repo.BranchName, form.NewBranchName)
	}
	if err != nil {
		if models.IsErrTagAlreadyExists(err) {
			e := err.(models.ErrTagAlreadyExists)
			ctx.Flash.Error(ctx.Tr("repo.branch.tag_collision", e.TagName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if models.IsErrBranchAlreadyExists(err) {
			e := err.(models.ErrBranchAlreadyExists)
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_already_exists", e.BranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if models.IsErrBranchNameConflict(err) {
			e := err.(models.ErrBranchNameConflict)
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_name_conflict", form.NewBranchName, e.BranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}

		ctx.ServerError("CreateNewBranch", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.create_success", form.NewBranchName))
	ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(form.NewBranchName))
}
