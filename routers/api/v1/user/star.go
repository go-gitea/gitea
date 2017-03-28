// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// getStarredRepos returns the repos that the user with the specified userID has
// starred
func getStarredRepos(userID int64, private bool) ([]*api.Repository, error) {
	starredRepos, err := models.GetStarredRepos(userID, private)
	if err != nil {
		return nil, err
	}

	repos := make([]*api.Repository, len(starredRepos))
	for i, starred := range starredRepos {
		access, err := models.AccessLevel(userID, starred)
		if err != nil {
			return nil, err
		}
		repos[i] = starred.APIFormat(access)
	}
	return repos, nil
}

// GetStarredRepos returns the repos that the user specified by the APIContext
// has starred
func GetStarredRepos(ctx *context.APIContext) {
	// swagger:route GET /users/{username}/starred userListStarred
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: RepositoryList
	//       500: error

	user := GetUserByParams(ctx)
	private := user.ID == ctx.User.ID
	repos, err := getStarredRepos(user.ID, private)
	if err != nil {
		ctx.Error(500, "getStarredRepos", err)
	}
	ctx.JSON(200, &repos)
}

// GetMyStarredRepos returns the repos that the authenticated user has starred
func GetMyStarredRepos(ctx *context.APIContext) {
	// swagger:route GET /user/starred userCurrentListStarred
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: RepositoryList
	//       500: error

	repos, err := getStarredRepos(ctx.User.ID, true)
	if err != nil {
		ctx.Error(500, "getStarredRepos", err)
	}
	ctx.JSON(200, &repos)
}

// IsStarring returns whether the authenticated is starring the repo
func IsStarring(ctx *context.APIContext) {
	// swagger:route GET /user/starred/{username}/{reponame} userCurrentCheckStarring
	//
	//     Responses:
	//       204: empty
	//       404: notFound

	if models.IsStaring(ctx.User.ID, ctx.Repo.Repository.ID) {
		ctx.Status(204)
	} else {
		ctx.Status(404)
	}
}

// Star the repo specified in the APIContext, as the authenticated user
func Star(ctx *context.APIContext) {
	// swagger:route PUT /user/starred/{username}/{reponame} userCurrentPutStar
	//
	//     Responses:
	//       204: empty
	//       500: error

	err := models.StarRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.Error(500, "StarRepo", err)
		return
	}
	ctx.Status(204)
}

// Unstar the repo specified in the APIContext, as the authenticated user
func Unstar(ctx *context.APIContext) {
	// swagger:route DELETE /user/starred/{username}/{reponame} userCurrentDeleteStar
	//
	//     Responses:
	//       204: empty
	//       500: error

	err := models.StarRepo(ctx.User.ID, ctx.Repo.Repository.ID, false)
	if err != nil {
		ctx.Error(500, "StarRepo", err)
		return
	}
	ctx.Status(204)
}
