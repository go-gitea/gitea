// Copyright 2024 The Gitea Authors.
// SPDX-License-Identifier: MIT

package org

import (
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
)

func ListBlocks(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/blocks organization organizationListBlocks
	// ---
	// summary: List users blocked by the organization
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	shared.ListBlocks(ctx, ctx.Org.Organization.AsUser())
}

func CheckUserBlock(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/blocks/{username} organization organizationCheckUserBlock
	// ---
	// summary: Check if a user is blocked by the organization
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to check
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	shared.CheckUserBlock(ctx, ctx.Org.Organization.AsUser())
}

func BlockUser(ctx *context.APIContext) {
	// swagger:operation PUT /orgs/{org}/blocks/{username} organization organizationBlockUser
	// ---
	// summary: Block a user
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to block
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

	shared.BlockUser(ctx, ctx.Org.Organization.AsUser())
}

func UnblockUser(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/blocks/{username} organization organizationUnblockUser
	// ---
	// summary: Unblock a user
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to unblock
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	shared.UnblockUser(ctx, ctx.Doer, ctx.Org.Organization.AsUser())
}
