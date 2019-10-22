// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
)

// ListForks list a repository's forks
func ListForks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/forks repository listForks
	// ---
	// summary: List a repository's forks
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"
	forks, err := ctx.Repo.Repository.GetForks()
	if err != nil {
		ctx.Error(500, "GetForks", err)
		return
	}
	apiForks := make([]*api.Repository, len(forks))
	for i, fork := range forks {
		access, err := models.AccessLevel(ctx.User, fork)
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
	// swagger:operation POST /repos/{owner}/{repo}/forks repository createFork
	// ---
	// summary: Fork a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to fork
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to fork
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateForkOption"
	// responses:
	//   "202":
	//     "$ref": "#/responses/Repository"
	repo := ctx.Repo.Repository
	var forker *models.User // user/org that will own the fork
	if form.Organization == nil {
		forker = ctx.User
	} else {
		org, err := models.GetOrgByName(*form.Organization)
		if err != nil {
			if models.IsErrOrgNotExist(err) {
				ctx.Error(422, "", err)
			} else {
				ctx.Error(500, "GetOrgByName", err)
			}
			return
		}
		isMember, err := org.IsOrgMember(ctx.User.ID)
		if err != nil {
			ctx.ServerError("IsOrgMember", err)
			return
		} else if !isMember {
			ctx.Status(403)
			return
		}
		forker = org
	}
	fork, err := models.ForkRepository(ctx.User, forker, repo, repo.Name, repo.Description)
	if err != nil {
		ctx.Error(500, "ForkRepository", err)
		return
	}
	ctx.JSON(202, fork.APIFormat(models.AccessModeOwner))
}
