// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/convert"
)

// GenerateRunnerToken generate a new runner token for a repository.
func GenerateRunnerToken(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/runners repository repoGenerateRunnerToken
	// ---
	// summary: Generate a new runner token for a repository.
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
	//     "$ref": "#/responses/ActionRunnerToken"
	//   "404":
	//     "$ref": "#/responses/notFound"

	token, err := actions_model.GetUnactivatedRunnerToken(ctx, ctx.Repo.Owner.ID, ctx.Repo.Repository.ID)
	if errors.Is(err, util.ErrNotExist) {
		token, err = actions_model.NewRunnerToken(ctx, ctx.Repo.Owner.ID, ctx.Repo.Repository.ID)
		if err != nil {
			ctx.ServerError("CreateRunnerToken", err)
			return
		}
	} else if err != nil {
		ctx.ServerError("GetUnactivatedRunnerToken", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIActionRunnerToken(token))
}
