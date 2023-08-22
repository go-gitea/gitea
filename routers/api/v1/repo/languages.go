// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
)

// GetPrimaryLanguageList returns a list of primary languages along with their color
func GetPrimaryLanguageList(ctx *context.APIContext) {
	// swagger:operation GET /repos/languages repository repoGetPrimaryLanguages
	// ---
	// summary: Get primary languages and their color
	// produces:
	//   - application/json
	// parameters:
	// - name: ids
	//   in: query
	//   description: Filter by Repo IDs (empty for all repos)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: integer
	// - name: ownerId
	//   in: query
	//   description: Filter by the the recent languages used by a user/org (ignored if filtered by Repo IDs)
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/PrimaryLanguageList"

	ids := ctx.FormInt64s("ids")
	var repos repo_model.RepositoryList

	ownerID := ctx.FormInt64("ownerId")

	if ownerID > 0 && len(ids) == 0 {
		userRepos, _, err := repo_model.GetUserRepositories(&repo_model.SearchRepoOptions{
			Actor: &user_model.User{
				ID: ownerID,
			},
			Private: ctx.IsSigned,
			ListOptions: db.ListOptions{
				Page:     0,
				PageSize: 20,
			},
		})
		if err != nil {
			ctx.InternalServerError(err)
		}

		repos = userRepos
	}

	if len(repos) == 0 {
		if len(ids) > 0 {
			repos = make([]*repo_model.Repository, len(ids))
			for i, id := range ids {
				repos[i] = &repo_model.Repository{ID: id}
			}
		} else {
			repos = nil
		}
	}

	langs, err := repo_model.GetPrimaryRepoLanguageList(ctx, repos)
	if err != nil {
		ctx.InternalServerError(err)
	}

	list := map[string]string{}

	for _, i := range langs {
		list[i.Language] = i.Color
	}

	ctx.JSON(http.StatusOK, list)
}
