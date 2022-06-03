// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
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

// PushMirrorAdd adds a push mirror to the repository
func PushMirrorAdd(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/push-mirrors repository repoPushMirrorAdd
	// ---
	// summary: Add a push mirror to the repository
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
	// - name: push_mirror_address
	//   in: query
	//   description: address of the push mirror
	//   type: string
	//   required: true
	// - name: push_mirror_username
	//   in: query
	//   description: The username of the push mirror
	//   type: string
	//   required: true
	// - name: push_mirror_password
	//   in: query
	//   description: The password of the push mirror
	//   type: string
	//   required: true
	// - name: push_mirror_interval
	//   in: query
	//   description: The interval of the push mirror
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := ctx.Repo.Repository
	opts := *web.GetForm(ctx).(*api.AddRepoPushMirrorOption)

	if !ctx.Repo.CanWrite(unit.TypeCode) {
		ctx.Error(http.StatusForbidden, "PushMirrorAdd", "Must have write access")
	}

	if setting.Mirror.DisableNewPush {
		ctx.Error(http.StatusBadRequest, "PushMirrorAdd", "New push is disabled")
		return
	}

	interval, err := time.ParseDuration(opts.PushMirrorInterval)
	if err != nil || (interval != 0 && interval < setting.Mirror.MinInterval) {
		ctx.Error(http.StatusUnprocessableEntity, "push_mirror_interval is too short", err)
		return
	}

	address, err := forms.ParseRemoteAddr(opts.PushMirrorAddress, opts.PushMirrorUsername, opts.PushMirrorPassword)
	if err == nil {
		err = migrations.IsMigrateURLAllowed(address, ctx.Doer)
	}
	if err != nil {
		if models.IsErrInvalidCloneAddr(err) {
			addrErr := err.(*models.ErrInvalidCloneAddr)
			switch {
			case addrErr.IsProtocolInvalid:
				ctx.Error(http.StatusUnprocessableEntity, "protocol of push_mirror_address is invalid", err)
			case addrErr.IsURLError:
				ctx.Error(http.StatusUnprocessableEntity, "url is not valid", err)
			case addrErr.IsPermissionDenied:
				ctx.Error(http.StatusUnprocessableEntity, "permission denied", err)
			case addrErr.IsInvalidPath:
				ctx.Error(http.StatusUnprocessableEntity, "local path is not valid", err)
			default:
				ctx.Error(http.StatusUnprocessableEntity, "Unknown error", err)
			}
			return
		}
		ctx.Error(http.StatusUnprocessableEntity, "push_mirror_address is not valid", err)
		return
	}

	remoteSuffix, err := util.CryptoRandomString(10)
	if err != nil {
		ctx.ServerError("RandomString", err)
		return
	}

	m := &repo_model.PushMirror{
		RepoID:     repo.ID,
		Repo:       repo,
		RemoteName: fmt.Sprintf("remote_mirror_%s", remoteSuffix),
		Interval:   interval,
	}
	if err := repo_model.InsertPushMirror(m); err != nil {
		ctx.ServerError("InsertPushMirror", err)
		return
	}

	if err := mirror_service.AddPushMirrorRemote(ctx, m, address); err != nil {
		if err := repo_model.DeletePushMirrorByID(m.ID); err != nil {
			log.Error("DeletePushMirrorByID %v", err)
		}
		ctx.ServerError("AddPushMirrorRemote", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToPushMirror(m, ctx.Repo.AccessMode))
}

// PushMirrorAdd removes a push mirror from the repository
func PushMirrorRemove(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/push-mirrors/{id} repository repoPushMirrorRemove
	// ---
	// summary: Remove a push mirror from the repository
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
	//   description: id of the push mirror
	//   type: string
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "PushMirrorRemove", "Mirror is disabled")
		return
	}

	m, err := getPushMirror(ctx)
	if err != nil {
		ctx.Error(http.StatusNotFound, "push mirror not found", err)
	}

	if err = mirror_service.RemovePushMirrorRemote(ctx, m); err != nil {
		ctx.ServerError("RemovePushMirrorRemote", err)
		return
	}

	if err = repo_model.DeletePushMirrorByID(m.ID); err != nil {
		ctx.ServerError("DeletePushMirrorByID", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToPushMirror(m, ctx.Repo.AccessMode))
}

// PushMirrorSync adds a push mirror to the queue
func PushMirrorSync(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/push-mirrors/{id}/sync repository repoPushMirrorSync
	// ---
	// summary: Sync a push mirror
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
	//   description: id of the push mirror
	//   type: string
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !setting.Mirror.Enabled {
		ctx.Error(http.StatusBadRequest, "PushMirrorSync", "Mirror is disabled")
		return
	}

	m, err := getPushMirror(ctx)
	if err != nil {
		ctx.Error(http.StatusNotFound, "push mirror not found", err)
	}

	mirror_service.AddPushMirrorToQueue(m.ID)
}

func getPushMirror(ctx *context.APIContext) (*repo_model.PushMirror, error) {
	id, err := strconv.ParseInt(ctx.Params(":id"), 10, 64)
	if err != nil {
		return nil, err
	}
	repo := ctx.Repo.Repository

	mirrors, err := repo_model.GetPushMirrorsByRepoID(repo.ID)
	if err != nil {
		return nil, err
	}
	for _, mirror := range mirrors {
		if mirror.ID == id {
			return mirror, nil
		}
	}

	return nil, fmt.Errorf("PushMirror[%v] not associated to repository %v", id, repo)
}
