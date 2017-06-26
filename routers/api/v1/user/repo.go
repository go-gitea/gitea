// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/sdk/gitea"
)

// listUserRepos - List the repositories owned by the given user.
func listUserRepos(ctx *context.APIContext, u *models.User) {
	showPrivateRepos := ctx.IsSigned && (ctx.User.ID == u.ID || ctx.User.IsAdmin)
	repos, err := models.GetUserRepositories(u.ID, showPrivateRepos, 1, u.NumRepos, "")
	if err != nil {
		ctx.Error(500, "GetUserRepositories", err)
		return
	}
	apiRepos := make([]*api.Repository, len(repos))
	var ctxUserID int64
	if ctx.User != nil {
		ctxUserID = ctx.User.ID
	}
	for i := range repos {
		access, err := models.AccessLevel(ctxUserID, repos[i])
		if err != nil {
			ctx.Error(500, "AccessLevel", err)
			return
		}
		apiRepos[i] = repos[i].APIFormat(access)
	}
	ctx.JSON(200, &apiRepos)
}

// ListUserRepos - list the repos owned by the given user.
func ListUserRepos(ctx *context.APIContext) {
	// swagger:route GET /users/{username}/repos userListRepos
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: RepositoryList
	//       500: error

	user := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserRepos(ctx, user)
}

// ListMyRepos - list the repositories you own or have access to.
func ListMyRepos(ctx *context.APIContext) {
	// swagger:route GET /user/repos userCurrentListRepos
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: RepositoryList
	//       500: error
	ownRepos, err := models.GetUserRepositories(ctx.User.ID, true, 1, ctx.User.NumRepos, "")
	if err != nil {
		ctx.Error(500, "GetUserRepositories", err)
		return
	}
	accessibleReposMap, err := ctx.User.GetRepositoryAccesses()
	if err != nil {
		ctx.Error(500, "GetRepositoryAccesses", err)
		return
	}

	apiRepos := make([]*api.Repository, len(ownRepos)+len(accessibleReposMap))
	for i := range ownRepos {
		apiRepos[i] = ownRepos[i].APIFormat(models.AccessModeOwner)
	}
	i := len(ownRepos)
	for repo, access := range accessibleReposMap {
		apiRepos[i] = repo.APIFormat(access)
		i++
	}
	ctx.JSON(200, &apiRepos)
}

// ListOrgRepos - list the repositories of an organization.
func ListOrgRepos(ctx *context.APIContext) {
	// swagger:route GET /orgs/{org}/repos orgListRepos
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: RepositoryList
	//       500: error

	listUserRepos(ctx, ctx.Org.Organization)
}
