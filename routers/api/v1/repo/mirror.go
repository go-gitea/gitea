// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/routers/api/v1/utils"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/migrations"
	mirror_service "code.gitea.io/gitea/services/mirror"
)

// MirrorSync adds a mirrored repository to the sync queue
func MirrorSync(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/mirror-sync repository repoMirrorSync
	// ---
	// summary: Sync a mirrored repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to sync
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to sync
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := ctx.Repo.Repository

	if !ctx.Repo.CanWrite(unit.TypeCode) {
		ctx.Error(http.StatusForbidden, "MirrorSync", "Must have write access")
	}

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "MirrorSync", "Mirror feature is disabled")
		return
	}

	if _, err := repo_model.GetMirrorByRepoID(ctx, repo.ID); err != nil {
		if errors.Is(err, repo_model.ErrMirrorNotExist) {
			ctx.Error(http.StatusBadRequest, "MirrorSync", "Repository is not a mirror")
			return
		}
		ctx.Error(http.StatusInternalServerError, "MirrorSync", err)
		return
	}

	mirror_service.StartToMirror(repo.ID)

	ctx.Status(http.StatusOK)
}

// PushMirrorSync adds all push mirrored repositories to the sync queue
func PushMirrorSync(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/push-mirror-sync repository repoPushMirrorSync
	// ---
	// summary: Sync all push mirrored repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to sync
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to sync
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := ctx.Repo.Repository

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "PushMirrorSync", "Mirror feature is disabled")
		return
	}
	// Get All push mirrors of a specific repo
	pushMirrors, err := repo_model.GetPushMirrorsByRepoID(repo.ID, db.ListOptions{})
	if err != nil {
		ctx.Error(http.StatusBadRequest, "PushMirrorSync", err)
		return
	}
	for _, mirror := range pushMirrors {
		mirror_service.SyncPushMirror(ctx, mirror.ID)
	}

	ctx.Status(http.StatusOK)
}

// ListPushMirrors get list of push mirrors of a repository
func ListPushMirrors(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/push-mirror repository repoListPushMirrors
	// ---
	// summary: Get all push mirrors of the repository
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/PushMirrorList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := ctx.Repo.Repository
	listOptions := utils.GetListOptions(ctx)
	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "ListPushMirrors", "Mirror feature is disabled")
		return
	}
	// Get All push mirrors of a specific repo
	pushMirrors, err := repo_model.GetPushMirrorsByRepoID(repo.ID, listOptions)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "ListPushMirrors", err)
		return
	}

	responsePushMirrors := make([]*api.PushMirror, 0)
	for _, mirror := range pushMirrors {
		responsePushMirrors = append(responsePushMirrors, convert.ToPushMirror(mirror, repo))
	}
	ctx.SetLinkHeader(len(pushMirrors), listOptions.PageSize)
	ctx.SetTotalCountHeader(int64(len(pushMirrors)))
	ctx.JSON(http.StatusOK, responsePushMirrors)
}

// GetPushMirrorByID get push mirror of a repository by ID
func GetPushMirrorByID(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/push-mirror/{id} repository repoGetPushMirrorByID
	// ---
	// summary: Get push mirror of the repository by ID
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to sync
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to sync
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: ID of push mirror
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PushMirror"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := ctx.Repo.Repository
	id := ctx.ParamsInt64(":id")
	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "GetPushMirrorByID", "Mirror feature is disabled")
		return
	}
	// Get push mirror of a specific repo by ID
	pushMirror, err := repo_model.GetPushMirrorByID(id)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "GetPushMirrorByID", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToPushMirror(pushMirror, repo))
}

// AddPushMirror adds a push mirror to a repository
func AddPushMirror(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/push-mirror repository repoAddPushMirror
	// ---
	// summary: add a push mirror to the repository
	// consumes:
	// - application/json
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreatePushMirrorOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PushMirror"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "400":
	//     "$ref": "#/responses/error"

	pushMirror := web.GetForm(ctx).(*api.CreatePushMirrorOption)

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "AddPushMirror", "Mirror feature is disabled")
		return
	}
	CreatePushMirror(ctx, pushMirror)
}

// DeletePushMirrorByID deletes a push mirror from a repository by ID
func DeletePushMirrorByID(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/push-mirror/{id} repository repoDeletePushMirror
	// ---
	// summary: deletes a push mirror from a repository by ID
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
	// - name: id
	//   in: path
	//   description: id of the pushMirror
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "400":
	//     "$ref": "#/responses/error"

	id := ctx.ParamsInt64(":id")

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "DeletePushMirrorByID", "Mirror feature is disabled")
		return
	}
	// delete push mirror by id
	err := repo_model.DeletePushMirrorByID(id)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "DeletePushMirrorByID", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func CreatePushMirror(ctx *context.APIContext, mirrorOption *api.CreatePushMirrorOption) {
	repo := ctx.Repo.Repository

	interval, err := time.ParseDuration(mirrorOption.Interval)
	if err != nil || (interval != 0 && interval < setting.Mirror.MinInterval) {
		ctx.Error(http.StatusBadRequest, "AddPushMirror", err)
		return
	}

	address, err := forms.ParseRemoteAddr(mirrorOption.RemoteAddress, mirrorOption.RemoteUsername, mirrorOption.RemotePassword)
	if err == nil {
		err = migrations.IsMigrateURLAllowed(address, ctx.ContextUser)
	}
	if err != nil {
		handleSettingRemoteAddrError(ctx, err)
		return
	}

	remoteSuffix, err := util.CryptoRandomString(10)
	if err != nil {
		ctx.ServerError("RandomString", err)
		return
	}

	pushMirror := &repo_model.PushMirror{
		RepoID:     repo.ID,
		Repo:       repo,
		RemoteName: fmt.Sprintf("remote_mirror_%s", remoteSuffix),
		Interval:   interval,
	}

	if err = repo_model.InsertPushMirror(pushMirror); err != nil {
		ctx.ServerError("InsertPushMirror", err)
		return
	}

	// if the registration of the push mirrorOption fails remove it from the database
	if err = mirror_service.AddPushMirrorRemote(ctx, pushMirror, address); err != nil {
		if err := repo_model.DeletePushMirrorByID(pushMirror.ID); err != nil {
			ctx.ServerError("AddPushMirror", err)
		}
		ctx.ServerError("AddPushMirror", err)
		return
	}

	// create response
	ctx.JSON(http.StatusCreated, convert.ToPushMirror(pushMirror, repo))
}

func handleSettingRemoteAddrError(ctx *context.APIContext, err error) {
	if models.IsErrInvalidCloneAddr(err) {
		addrErr := err.(*models.ErrInvalidCloneAddr)
		switch {
		case addrErr.IsProtocolInvalid:
			ctx.Error(http.StatusBadRequest, "AddPushMirror", "Invalid mirror protocol")
		case addrErr.IsURLError:
			ctx.Error(http.StatusBadRequest, "AddPushMirror", "Invalid Url ")
		case addrErr.IsPermissionDenied:
			ctx.Error(http.StatusUnauthorized, "AddPushMirror", "permission denied")
		default:
			ctx.Error(http.StatusBadRequest, "AddPushMirror", "Unknown error")
		}
		return
	}
}
