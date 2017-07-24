// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/convert"
)

// GetBranch get a branch of a repository
// see https://github.com/gogits/go-gogs-client/wiki/Repositories#get-branch
func GetBranch(ctx *context.APIContext) {
	if ctx.Repo.TreePath != "" {
		// if TreePath != "", then URL contained extra slashes
		// (i.e. "master/subbranch" instead of "master"), so branch does
		// not exist
		ctx.Status(404)
		return
	}
	branch, err := ctx.Repo.Repository.GetBranch(ctx.Repo.BranchName)
	if err != nil {
		if models.IsErrBranchNotExist(err) {
			ctx.Error(404, "GetBranch", err)
		} else {
			ctx.Error(500, "GetBranch", err)
		}
		return
	}

	c, err := branch.GetCommit()
	if err != nil {
		ctx.Error(500, "GetCommit", err)
		return
	}

	ctx.JSON(200, convert.ToBranch(branch, c))
}

// ListBranches list all the branches of a repository
// see https://github.com/gogits/go-gogs-client/wiki/Repositories#list-branches
func ListBranches(ctx *context.APIContext) {
	branches, err := ctx.Repo.Repository.GetBranches()
	if err != nil {
		ctx.Error(500, "GetBranches", err)
		return
	}

	apiBranches := make([]*api.Branch, len(branches))
	for i := range branches {
		c, err := branches[i].GetCommit()
		if err != nil {
			ctx.Error(500, "GetCommit", err)
			return
		}
		apiBranches[i] = convert.ToBranch(branches[i], c)
	}

	ctx.JSON(200, &apiBranches)
}
