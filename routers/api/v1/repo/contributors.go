// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// ListContributors lists repository contributors.
func ListContributors(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contributors repository repoListContributors
	// ---
	// summary: List repository contributors
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
	// - name: anon
	//   in: query
	//   description: include anonymous contributors
	//   type: boolean
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
	//     "$ref": "#/responses/ContributorList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	includeAnonymous := ctx.FormBool("anon")
	contributors, total, err := repo_model.GetRepoContributors(ctx, ctx.Repo.Repository.ID, includeAnonymous, utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	userIDs := make(map[int64]struct{})
	for _, contributor := range contributors {
		if contributor.UserID > 0 {
			userIDs[contributor.UserID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}
	users, err := user_model.GetUsersByIDs(ctx, ids)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	userMap := make(map[int64]*user_model.User, len(users))
	for _, user := range users {
		userMap[user.ID] = user
	}

	result := make([]*api.Contributor, 0, len(contributors))
	for _, contributor := range contributors {
		if user := userMap[contributor.UserID]; user != nil {
			result = append(result, &api.Contributor{
				User:          convert.ToUser(ctx, user, ctx.Doer),
				Contributions: contributor.Commits,
			})
			continue
		}
		result = append(result, &api.Contributor{
			Name:          contributor.AuthorName,
			Email:         contributor.Email,
			Contributions: contributor.Commits,
		})
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, result)
}
