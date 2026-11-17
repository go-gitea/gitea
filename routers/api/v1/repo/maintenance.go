// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	repo_module "gitea.dev/modules/repository"
	"gitea.dev/services/context"
)

// MaintenanceSyncBranches synchronizes the branches of a repository with its git data
func MaintenanceSyncBranches(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/maintenance/sync-branches repository repoMaintenanceSyncBranches
	// ---
	// summary: Synchronize the branches of a repository with its git data
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"

	if _, err := repo_module.SyncRepoBranches(ctx, ctx.Repo.Repository.ID, 0); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// MaintenanceSyncTags synchronizes the tags of a repository with its releases
func MaintenanceSyncTags(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/maintenance/sync-tags repository repoMaintenanceSyncTags
	// ---
	// summary: Synchronize the tags of a repository with its git data
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"

	if err := repo_module.SyncRepoTags(ctx, ctx.Repo.Repository.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
