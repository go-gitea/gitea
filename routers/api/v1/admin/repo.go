// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/repo"
)

// CreateRepo api for creating a repository
func CreateRepo(ctx *context.APIContext) {
	// swagger:operation POST /admin/users/{username}/repos admin adminCreateRepo
	// ---
	// summary: Create a repository on behalf of a user
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user. This user will own the created repository
	//   type: string
	//   required: true
	// - name: repository
	//   in: body
	//   required: true
	//   schema: { "$ref": "#/definitions/CreateRepoOption" }
	// responses:
	//   "201":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateRepoOption)

	repo.CreateUserRepo(ctx, ctx.ContextUser, *form)
}
