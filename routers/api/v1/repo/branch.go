// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "github.com/gogits/go-gogs-client"

	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/routers/api/v1/convert"
)

// https://github.com/gogits/go-gogs-client/wiki/Repositories#get-branch
func GetBranch(ctx *context.APIContext) {
	branch, err := ctx.Repo.Repository.GetBranch(ctx.Params(":branchname"))
	if err != nil {
		ctx.Error(500, "GetBranch", err)
		return
	}

	c, err := branch.GetCommit()
	if err != nil {
		ctx.Error(500, "GetCommit", err)
		return
	}

	ctx.JSON(200, convert.ToBranch(branch, c))
}

// https://github.com/gogits/go-gogs-client/wiki/Repositories#list-branches
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
