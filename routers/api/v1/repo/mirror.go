// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
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
	//   "404":
	//     "$ref": "#/responses/notFound"

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

	mirror_service.AddPullMirrorToQueue(repo.ID)

	ctx.Status(http.StatusOK)
}

// PushMirrorSync adds all push mirrored repositories to the sync queue
func PushMirrorSync(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/push_mirrors-sync repository repoPushMirrorSync
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
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "PushMirrorSync", "Mirror feature is disabled")
		return
	}
	// Get All push mirrors of a specific repo
	pushMirrors, _, err := repo_model.GetPushMirrorsByRepoID(ctx, ctx.Repo.Repository.ID, db.ListOptions{})
	if err != nil {
		ctx.Error(http.StatusNotFound, "PushMirrorSync", err)
		return
	}
	for _, mirror := range pushMirrors {
		ok := mirror_service.SyncPushMirror(ctx, mirror.ID)
		if !ok {
			ctx.Error(http.StatusInternalServerError, "PushMirrorSync", "error occurred when syncing push mirror "+mirror.RemoteName)
			return
		}
	}

	ctx.Status(http.StatusOK)
}

// ListPushMirrors get list of push mirrors of a repository
func ListPushMirrors(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/push_mirrors repository repoListPushMirrors
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "GetPushMirrorsByRepoID", "Mirror feature is disabled")
		return
	}

	repo := ctx.Repo.Repository
	// Get all push mirrors for the specified repository.
	pushMirrors, count, err := repo_model.GetPushMirrorsByRepoID(ctx, repo.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusNotFound, "GetPushMirrorsByRepoID", err)
		return
	}

	responsePushMirrors := make([]*api.PushMirror, 0, len(pushMirrors))
	for _, mirror := range pushMirrors {
		m, err := convert.ToPushMirror(ctx, mirror)
		if err == nil {
			responsePushMirrors = append(responsePushMirrors, m)
		}
	}
	ctx.SetLinkHeader(len(responsePushMirrors), utils.GetListOptions(ctx).PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, responsePushMirrors)
}

// GetPushMirrorByName get push mirror of a repository by name
func GetPushMirrorByName(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/push_mirrors/{name} repository repoGetPushMirrorByRemoteName
	// ---
	// summary: Get push mirror of the repository by remoteName
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
	// - name: name
	//   in: path
	//   description: remote name of push mirror
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PushMirror"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "GetPushMirrorByRemoteName", "Mirror feature is disabled")
		return
	}

	mirrorName := ctx.Params(":name")
	// Get push mirror of a specific repo by remoteName
	pushMirror, exist, err := db.Get[repo_model.PushMirror](ctx, repo_model.PushMirrorOptions{
		RepoID:     ctx.Repo.Repository.ID,
		RemoteName: mirrorName,
	}.ToConds())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPushMirrors", err)
		return
	} else if !exist {
		ctx.Error(http.StatusNotFound, "GetPushMirrors", nil)
		return
	}

	m, err := convert.ToPushMirror(ctx, pushMirror)
	if err != nil {
		ctx.ServerError("GetPushMirrorByRemoteName", err)
		return
	}
	ctx.JSON(http.StatusOK, m)
}

// AddPushMirror adds a push mirror to a repository
func AddPushMirror(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/push_mirrors repository repoAddPushMirror
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
	//   "200":
	//     "$ref": "#/responses/PushMirror"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "AddPushMirror", "Mirror feature is disabled")
		return
	}

	pushMirror := web.GetForm(ctx).(*api.CreatePushMirrorOption)
	CreatePushMirror(ctx, pushMirror)
}

// DeletePushMirrorByRemoteName deletes a push mirror from a repository by remoteName
func DeletePushMirrorByRemoteName(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/push_mirrors/{name} repository repoDeletePushMirror
	// ---
	// summary: deletes a push mirror from a repository by remoteName
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
	// - name: name
	//   in: path
	//   description: remote name of the pushMirror
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "400":
	//     "$ref": "#/responses/error"

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "DeletePushMirrorByName", "Mirror feature is disabled")
		return
	}

	remoteName := ctx.Params(":name")
	// Delete push mirror on repo by name.
	err := repo_model.DeletePushMirrors(ctx, repo_model.PushMirrorOptions{RepoID: ctx.Repo.Repository.ID, RemoteName: remoteName})
	if err != nil {
		ctx.Error(http.StatusNotFound, "DeletePushMirrors", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func CreatePushMirror(ctx *context.APIContext, mirrorOption *api.CreatePushMirrorOption) {
	repo := ctx.Repo.Repository

	interval, err := time.ParseDuration(mirrorOption.Interval)
	if err != nil || (interval != 0 && interval < setting.Mirror.MinInterval) {
		ctx.Error(http.StatusBadRequest, "CreatePushMirror", err)
		return
	}

	address, err := forms.ParseRemoteAddr(mirrorOption.RemoteAddress, mirrorOption.RemoteUsername, mirrorOption.RemotePassword)
	if err == nil {
		err = migrations.IsMigrateURLAllowed(address, ctx.ContextUser)
	}
	if err != nil {
		HandleRemoteAddressError(ctx, err)
		return
	}

	remoteSuffix, err := util.CryptoRandomString(10)
	if err != nil {
		ctx.ServerError("CryptoRandomString", err)
		return
	}

	remoteAddress, err := util.SanitizeURL(mirrorOption.RemoteAddress)
	if err != nil {
		ctx.ServerError("SanitizeURL", err)
		return
	}

	pushMirror := &repo_model.PushMirror{
		RepoID:        repo.ID,
		Repo:          repo,
		RemoteName:    fmt.Sprintf("remote_mirror_%s", remoteSuffix),
		Interval:      interval,
		SyncOnCommit:  mirrorOption.SyncOnCommit,
		RemoteAddress: remoteAddress,
	}

	if err = db.Insert(ctx, pushMirror); err != nil {
		ctx.ServerError("InsertPushMirror", err)
		return
	}

	// if the registration of the push mirrorOption fails remove it from the database
	if err = mirror_service.AddPushMirrorRemote(ctx, pushMirror, address); err != nil {
		if err := repo_model.DeletePushMirrors(ctx, repo_model.PushMirrorOptions{ID: pushMirror.ID, RepoID: pushMirror.RepoID}); err != nil {
			ctx.ServerError("DeletePushMirrors", err)
			return
		}
		ctx.ServerError("AddPushMirrorRemote", err)
		return
	}
	m, err := convert.ToPushMirror(ctx, pushMirror)
	if err != nil {
		ctx.ServerError("ToPushMirror", err)
		return
	}
	ctx.JSON(http.StatusOK, m)
}

func HandleRemoteAddressError(ctx *context.APIContext, err error) {
	if models.IsErrInvalidCloneAddr(err) {
		addrErr := err.(*models.ErrInvalidCloneAddr)
		switch {
		case addrErr.IsProtocolInvalid:
			ctx.Error(http.StatusBadRequest, "CreatePushMirror", "Invalid mirror protocol")
		case addrErr.IsURLError:
			ctx.Error(http.StatusBadRequest, "CreatePushMirror", "Invalid Url ")
		case addrErr.IsPermissionDenied:
			ctx.Error(http.StatusUnauthorized, "CreatePushMirror", "Permission denied")
		default:
			ctx.Error(http.StatusBadRequest, "CreatePushMirror", "Unknown error")
		}
		return
	}
}
