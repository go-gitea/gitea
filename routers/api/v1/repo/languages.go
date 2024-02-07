// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
)

func listPrimaryLanguageList(ctx *context.APIContext, ownerID int64) {
	langs, err := repo_model.GetPrimaryRepoLanguageList(ctx, ownerID, ctx.Doer)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	list := []api.LanguageInfo{}

	for _, i := range langs {
		list = append(list, api.LanguageInfo{
			Name:  i.Language,
			Color: i.Color,
		})
	}

	ctx.JSON(http.StatusOK, list)
}

// List all primary languages
func ListPrimaryLanguages(ctx *context.APIContext) {
	// swagger:operation GET /repos/languages repository repoListPrimaryLanguages
	// ---
	// summary: Get primary languages and their color
	// produces:
	//   - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/PrimaryLanguageList"

	listPrimaryLanguageList(ctx, 0)
}

// List user primary languages
func GetUserPrimaryLanguageList(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/languages user userListPrimaryLanguages
	// ---
	// summary: Get primary languages and their color
	// produces:
	//   - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user to get
	//   type: string
	//   required: true
	// responses:
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "200":
	//     "$ref": "#/responses/PrimaryLanguageList"

	listPrimaryLanguageList(ctx, ctx.ContextUser.ID)
}

// List org primary languages
func GetOrgPrimaryLanguageList(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/languages organization orgListPrimaryLanguages
	// ---
	// summary: Get primary languages and their color
	// produces:
	//   - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization to get
	//   type: string
	//   required: true
	// responses:
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "200":
	//     "$ref": "#/responses/PrimaryLanguageList"

	listPrimaryLanguageList(ctx, ctx.Org.Organization.ID)
}
