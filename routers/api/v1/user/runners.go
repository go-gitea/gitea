// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
)

// https://docs.github.com/en/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-organization

// CreateRegistrationToken returns the token to register user runners
func CreateRegistrationToken(ctx *context.APIContext) {
	// swagger:operation POST /user/actions/runners/registration-token user userCreateRunnerRegistrationToken
	// ---
	// summary: Get an user's actions runner registration token
	// produces:
	// - application/json
	// parameters:
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, ctx.Doer.ID, 0)
}

// ListRunners get user-level runners
func ListRunners(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/runners user getUserRunners
	// ---
	// summary: Get user-level runners
	// produces:
	// - application/json
	// parameters:
	// - name: disabled
	//   in: query
	//   description: filter by disabled status (true or false)
	//   type: boolean
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunnerList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.ListRunners(ctx, ctx.Doer.ID, 0)
}

// GetRunner get a user-level runner
func GetRunner(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/runners/{runner_id} user getUserRunner
	// ---
	// summary: Get a user-level runner
	// produces:
	// - application/json
	// parameters:
	// - name: runner_id
	//   in: path
	//   description: id of the runner
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Runner"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.GetRunner(ctx, ctx.Doer.ID, 0, ctx.PathParamInt64("runner_id"))
}

// DeleteRunner delete a user-level runner
func DeleteRunner(ctx *context.APIContext) {
	// swagger:operation DELETE /user/actions/runners/{runner_id} user deleteUserRunner
	// ---
	// summary: Delete a user-level runner
	// produces:
	// - application/json
	// parameters:
	// - name: runner_id
	//   in: path
	//   description: id of the runner
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: runner has been deleted
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.DeleteRunner(ctx, ctx.Doer.ID, 0, ctx.PathParamInt64("runner_id"))
}

// UpdateRunner update a user-level runner
func UpdateRunner(ctx *context.APIContext) {
	// swagger:operation PATCH /user/actions/runners/{runner_id} user updateUserRunner
	// ---
	// summary: Update a user-level runner
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: runner_id
	//   in: path
	//   description: id of the runner
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditActionRunnerOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Runner"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	shared.UpdateRunner(ctx, ctx.Doer.ID, 0, ctx.PathParamInt64("runner_id"))
}
