// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// CodeSearch Performs a code search on all Repos
func CodeSearch(ctx *context.APIContext) {
	// swagger:operation GET /code_search miscellaneous globalCodeSearch
	// ---
	// summary: Performs a code search on all Repos
	// produces:
	// - application/json
	// parameters:
	// - name: keyword
	//   in: query
	//   description: the keyword the search for
	//   type: string
	//   required: true
	// - name: language
	//   in: query
	//   description: filter results by language
	//   type: string
	// - name: match
	//   in: query
	//   description: only exact match (defaults to false)
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserHeatmapData"
	//   "422":
	//     description: "The keyword is empty"
	//   "501":
	//     description: "The repo indexer is disabled for this instance"
	repos, err := repo_model.FindUserCodeAccessibleRepoIDs(ctx, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindUserCodeAccessibleRepoIDs", err)
		return
	}

	utils.PerformCodeSearch(ctx, repos)
}
