// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplBranch base.TplName = "repo/branch"
)

// Branches render repository branch page
func Branches(ctx *context.Context) {
	ctx.Data["Title"] = "Branches"
	ctx.Data["IsRepoToolbarBranches"] = true

	brs, err := ctx.Repo.GitRepo.GetBranches()
	if err != nil {
		ctx.Handle(500, "repo.Branches(GetBranches)", err)
		return
	} else if len(brs) == 0 {
		ctx.Handle(404, "repo.Branches(GetBranches)", nil)
		return
	}

	ctx.Data["Branches"] = brs
	ctx.HTML(200, tplBranch)
}

// CreateBranch creates new branch in repository
func CreateBranch(ctx *context.Context, form auth.NewBranchForm) {
	if !ctx.Repo.CanCreateBranch() {
		ctx.Handle(404, "CreateBranch", nil)
		return
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName)
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
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName)
			return
		}
		if models.IsErrBranchAlreadyExists(err) {
			e := err.(models.ErrBranchAlreadyExists)
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_already_exists", e.BranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName)
			return
		}
		if models.IsErrBranchNameConflict(err) {
			e := err.(models.ErrBranchNameConflict)
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_name_conflict", form.NewBranchName, e.BranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName)
			return
		}

		ctx.Handle(500, "CreateNewBranch", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.create_success", form.NewBranchName))
	ctx.Redirect(ctx.Repo.RepoLink + "/src/" + form.NewBranchName)
}
