// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
)

// https://docs.github.com/en/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-organization

// GetRegistrationToken returns the token to register user runners
func GetRegistrationToken(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/runners/registration-token user userGetRunnerRegistrationToken
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
