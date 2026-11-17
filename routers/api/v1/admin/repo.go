// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	repo_model "gitea.dev/models/repo"
	repo_module "gitea.dev/modules/repository"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/repo"
	"gitea.dev/services/context"
)

// CreateRepo api for creating a repository
func CreateRepo(ctx *context.APIContext) {
	// swagger:operation POST /admin/users/{username}/repos admin adminCreateRepo
	// ---
	// summary: Create a repository on behalf of a user
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user who will own the created repository
	//   type: string
	//   required: true
	// - name: repository
	//   in: body
	//   required: true
	//   schema: { "$ref": "#/definitions/CreateRepoOption" }
	// responses:
	//   "201":
	//     "$ref": "#/responses/Repository"
	//   default:
	//     "$ref": "#/responses/error"

	form := web.GetForm[*api.CreateRepoOption](ctx)

	repo.CreateUserRepo(ctx, ctx.ContextUser, *form)
}

func getRepoForSync(ctx *context.APIContext) *repo_model.Repository {
	dbRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ctx.PathParam("username"), ctx.PathParam("reponame"))
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil
	}
	return dbRepo
}

// SyncRepoBranches synchronizes the branches of a repository with the database
func SyncRepoBranches(ctx *context.APIContext) {
	// swagger:operation POST /admin/repos/{owner}/{repo}/sync-branches admin adminSyncRepoBranches
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

	dbRepo := getRepoForSync(ctx)
	if dbRepo == nil {
		return
	}
	if _, err := repo_module.SyncRepoBranches(ctx, dbRepo.ID, 0); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// SyncRepoTags synchronizes the tags of a repository with its releases
func SyncRepoTags(ctx *context.APIContext) {
	// swagger:operation POST /admin/repos/{owner}/{repo}/sync-tags admin adminSyncRepoTags
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

	dbRepo := getRepoForSync(ctx)
	if dbRepo == nil {
		return
	}
	if err := repo_module.SyncRepoTags(ctx, dbRepo.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
