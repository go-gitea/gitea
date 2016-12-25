// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
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

// DeleteBranchPost responses for delete merged branch
func DeleteBranchPost(ctx *context.Context) {
	branchName := ctx.Params(":name")
	commitID := ctx.Query("commit")

	defer func() {
		redirectTo := ctx.Query("redirect_to")
		if len(redirectTo) == 0 {
			redirectTo = ctx.Repo.RepoLink
		}

		ctx.JSON(200, map[string]interface{}{
			"redirect": redirectTo,
		})
	}()

	fullBranchName := ctx.Repo.Owner.Name + "/" + branchName

	if !ctx.Repo.GitRepo.IsBranchExist(branchName) || branchName == "master" {
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}

	if len(commitID) > 0 {
		branchCommitID, err := ctx.Repo.GitRepo.GetBranchCommitID(branchName)
		if err != nil {
			log.Error(4, "GetBranchCommitID: %v", err)
			return
		}

		if branchCommitID != commitID {
			ctx.Flash.Error(ctx.Tr("repo.branch.delete_branch_has_new_commits", fullBranchName))
			return
		}
	}

	if err := ctx.Repo.GitRepo.DeleteBranch(branchName, git.DeleteBranchOptions{
		Force: false,
	}); err != nil {
		log.Error(4, "DeleteBranch: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", fullBranchName))
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.deletion_success", fullBranchName))
}
