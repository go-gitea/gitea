// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ListStargazers list a repository's stargazers
func ListStargazers(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/stargazers repository repoListStargazers
	// ---
	// summary: List a repo's stargazers
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	stargazers, err := repo_model.GetStargazers(ctx.Repo.Repository, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetStargazers", err)
		return
	}
	users := make([]*api.User, len(stargazers))
	for i, stargazer := range stargazers {
		users[i] = convert.ToUser(stargazer, ctx.Doer)
	}

	ctx.SetTotalCountHeader(int64(ctx.Repo.Repository.NumStars))
	ctx.JSON(http.StatusOK, users)
}
