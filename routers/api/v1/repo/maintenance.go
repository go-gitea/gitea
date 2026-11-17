// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	"gitea.dev/modules/git/gitcmd"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"
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

// MaintenanceRemoveStaleLocks removes the stale git lock files of a repository
func MaintenanceRemoveStaleLocks(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/maintenance/remove-stale-locks repository repoMaintenanceRemoveStaleLocks
	// ---
	// summary: Remove the stale git lock files of a repository
	// description: Removes git lock files left behind by interrupted git processes, which have not been modified for at least one hour.
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

	if _, err := repo_service.RemoveStaleRepoLockFiles(ctx, ctx.Repo.Repository); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// MaintenanceGarbageCollect runs "git gc" on a repository, it responds with 409 Conflict if garbage collection is already running
func MaintenanceGarbageCollect(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/maintenance/gc repository repoMaintenanceGarbageCollect
	// ---
	// summary: Run garbage collection ("git gc") on a repository
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
	//   "409":
	//     "$ref": "#/responses/error"
	//   default:
	//     "$ref": "#/responses/error"

	// the git args are set by config, they can be safe to be trusted
	err := repo_service.GarbageCollectRepo(ctx, ctx.Repo.Repository, gitcmd.ToTrustedCmdArgs(setting.Git.GCArgs))
	if errors.Is(err, repo_service.ErrGitGcAlreadyRunning) {
		ctx.APIError(http.StatusConflict, err.Error())
		return
	} else if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
