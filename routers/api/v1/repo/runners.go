// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
)

// GetRegistrationToken returns the token to register repo runners
func GetRegistrationToken(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/runners/registration-token repository repoGetRunnerRegistrationToken
	// ---
	// summary: Get a repository's actions runner registration token
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, ctx.Repo.Repository.OwnerID, ctx.Repo.Repository.ID)
}
