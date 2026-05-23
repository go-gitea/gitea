// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
)

// https://docs.github.com/en/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-organization

// CreateRegistrationToken returns the token to register global runners
func CreateRegistrationToken(ctx *context.APIContext) {
	// swagger:operation POST /admin/actions/runners/registration-token admin adminCreateRunnerRegistrationToken
	// ---
	// summary: Get a global actions runner registration token
	// produces:
	// - application/json
	// parameters:
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, 0, 0)
}

// ListRunners get all runners
func ListRunners(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/runners admin getAdminRunners
	// ---
	// summary: Get all runners
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
	shared.ListRunners(ctx, 0, 0)
}

// GetRunner get a global runner
func GetRunner(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/runners/{runner_id} admin getAdminRunner
	// ---
	// summary: Get a global runner
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
	shared.GetRunner(ctx, 0, 0, ctx.PathParamInt64("runner_id"))
}

// DeleteRunner delete a global runner
func DeleteRunner(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/actions/runners/{runner_id} admin deleteAdminRunner
	// ---
	// summary: Delete a global runner
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
	shared.DeleteRunner(ctx, 0, 0, ctx.PathParamInt64("runner_id"))
}

// UpdateRunner update a global runner
func UpdateRunner(ctx *context.APIContext) {
	// swagger:operation PATCH /admin/actions/runners/{runner_id} admin updateAdminRunner
	// ---
	// summary: Update a global runner
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
	shared.UpdateRunner(ctx, 0, 0, ctx.PathParamInt64("runner_id"))
}
