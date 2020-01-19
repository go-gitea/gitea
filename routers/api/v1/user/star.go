// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
)

// getStarredRepos returns the repos that the user with the specified userID has
// starred
func getStarredRepos(user *models.User, private bool) ([]*api.Repository, error) {
	starredRepos, err := models.GetStarredRepos(user.ID, private)
	if err != nil {
		return nil, err
	}

	repos := make([]*api.Repository, len(starredRepos))
	for i, starred := range starredRepos {
		access, err := models.AccessLevel(user, starred)
		if err != nil {
			return nil, err
		}
		repos[i] = starred.APIFormat(access)
	}
	return repos, nil
}

// GetStarredRepos returns the repos that the given user has starred
func GetStarredRepos(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/starred user userListStarred
	// ---
	// summary: The repos that the given user has starred
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"

	user := GetUserByParams(ctx)
	private := user.ID == ctx.User.ID
	repos, err := getStarredRepos(user, private)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "getStarredRepos", err)
	}
	ctx.JSON(http.StatusOK, &repos)
}

// GetMyStarredRepos returns the repos that the authenticated user has starred
func GetMyStarredRepos(ctx *context.APIContext) {
	// swagger:operation GET /user/starred user userCurrentListStarred
	// ---
	// summary: The repos that the authenticated user has starred
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"

	repos, err := getStarredRepos(ctx.User, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "getStarredRepos", err)
	}
	ctx.JSON(http.StatusOK, &repos)
}

// IsStarring returns whether the authenticated is starring the repo
func IsStarring(ctx *context.APIContext) {
	// swagger:operation GET /user/starred/{owner}/{repo} user userCurrentCheckStarring
	// ---
	// summary: Whether the authenticated is starring the repo
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if models.IsStaring(ctx.User.ID, ctx.Repo.Repository.ID) {
		ctx.Status(http.StatusNoContent)
	} else {
		ctx.NotFound()
	}
}

// Star the repo specified in the APIContext, as the authenticated user
func Star(ctx *context.APIContext) {
	// swagger:operation PUT /user/starred/{owner}/{repo} user userCurrentPutStar
	// ---
	// summary: Star the given repo
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to star
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to star
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	err := models.StarRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "StarRepo", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// Unstar the repo specified in the APIContext, as the authenticated user
func Unstar(ctx *context.APIContext) {
	// swagger:operation DELETE /user/starred/{owner}/{repo} user userCurrentDeleteStar
	// ---
	// summary: Unstar the given repo
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to unstar
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to unstar
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	err := models.StarRepo(ctx.User.ID, ctx.Repo.Repository.ID, false)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "StarRepo", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
