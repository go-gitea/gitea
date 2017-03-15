// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// ListForks list a repository's forks
func ListForks(ctx *context.APIContext) {
	forks, err := ctx.Repo.Repository.GetForks()
	if err != nil {
		ctx.Error(500, "GetForks", err)
		return
	}
	apiForks := make([]*api.Repository, len(forks))
	for i, fork := range forks {
		access, err := models.AccessLevel(ctx.User.ID, fork)
		if err != nil {
			ctx.Error(500, "AccessLevel", err)
			return
		}
		apiForks[i] = fork.APIFormat(access)
	}
	ctx.JSON(200, apiForks)
}

// CreateFork create a fork of a repo
func CreateFork(ctx *context.APIContext, form api.CreateForkOption) {
	repo := ctx.Repo.Repository
	var forker *models.User // user/org that will own the fork
	if form.Organization == nil {
		forker = ctx.User
	} else {
		org, err := models.GetOrgByName(*form.Organization)
		if err != nil {
			if err == models.ErrOrgNotExist {
				ctx.Error(422, "", err)
			} else {
				ctx.Error(500, "GetOrgByName", err)
			}
			return
		}
		if !org.IsOrgMember(ctx.User.ID) {
			ctx.Status(403)
			return
		}
		forker = org
	}
	fork, err := models.ForkRepository(forker, repo, repo.Name, repo.Description)
	if err != nil {
		ctx.Error(500, "ForkRepository", err)
		return
	}
	ctx.JSON(202, fork.APIFormat(models.AccessModeOwner))
}
