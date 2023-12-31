// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/shared"
)

// https://docs.github.com/en/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-organization

// GetRegistrationToken returns the token to register org runners
func GetRegistrationToken(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/runners/registration-token organization orgGetRunnerRegistrationToken
	// ---
	// summary: Get an organization's actions runner registration token
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, ctx.Org.Organization.ID, 0)
}
