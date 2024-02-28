// Copyright 2024 The Gitea Authors.
// SPDX-License-Identifier: MIT

package user

import (
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
)

func ListBlocks(ctx *context.APIContext) {
	// swagger:operation GET /user/blocks user userListBlocks
	// ---
	// summary: List users blocked by the authenticated user
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	shared.ListBlocks(ctx, ctx.Doer)
}

func CheckUserBlock(ctx *context.APIContext) {
	// swagger:operation GET /user/blocks/{username} user userCheckUserBlock
	// ---
	// summary: Check if a user is blocked by the authenticated user
	// parameters:
	// - name: username
	//   in: path
	//   description: user to check
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	shared.CheckUserBlock(ctx, ctx.Doer)
}

func BlockUser(ctx *context.APIContext) {
	// swagger:operation PUT /user/blocks/{username} user userBlockUser
	// ---
	// summary: Block a user
	// parameters:
	// - name: username
	//   in: path
	//   description: user to block
	//   type: string
	//   required: true
	// - name: note
	//   in: query
	//   description: optional note for the block
	//   type: string
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	shared.BlockUser(ctx, ctx.Doer)
}

func UnblockUser(ctx *context.APIContext) {
	// swagger:operation DELETE /user/blocks/{username} user userUnblockUser
	// ---
	// summary: Unblock a user
	// parameters:
	// - name: username
	//   in: path
	//   description: user to unblock
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	shared.UnblockUser(ctx, ctx.Doer, ctx.Doer)
}
