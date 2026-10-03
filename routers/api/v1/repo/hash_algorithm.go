// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	api "gitea.dev/modules/structs"
	"gitea.dev/services/context"
)

func GetHashAlgorithm(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/hash-algorithm repository repoGetHashAlgorithm
	// ---
	// summary: Get the hash algorithm used to store repository objects
	// produces:
	//   - application/json
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
	//     "$ref": "#/responses/RepoHashAlgorithm"
	//   "404":
	//     "$ref": "#/responses/notFound"

	ctx.JSON(http.StatusOK, api.RepoHashAlgorithm{HashAlgorithm: api.ObjectFormatName(ctx.Repo.Repository.ObjectFormatName)})
}
