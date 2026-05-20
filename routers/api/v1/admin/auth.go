// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// SearchAuth API for getting information of the configured authentication methods according the filter conditions
func SearchAuth(ctx *context.APIContext) {
	// swagger:operation GET /admin/identity-auth admin adminSearchAuth
	// ---
	// summary: Search authentication sources
	// produces:
	// - application/json
	// parameters:
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
	//     description: "SearchResults of authentication sources"
	//     schema:
	//       type: array
	//       items:
	//         "$ref": "#/definitions/AuthOauth2Option"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	listOptions := utils.GetListOptions(ctx)

	authSources, maxResults, err := db.FindAndCount[auth_model.Source](ctx, auth_model.FindSourcesOptions{})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	results := make([]*api.AuthSourceOption, len(authSources))
	for i := range authSources {
		results[i] = convert.ToOauthProvider(ctx, authSources[i])
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &results)
}
