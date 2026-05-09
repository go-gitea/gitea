// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"net/url"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

func composeHTTPSDeployKeysAPILink(owner, name string) string {
	return setting.AppURL + "api/v1/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/https_keys/"
}

// ListHTTPSDeployKeys list all the HTTPS deploy keys of a repository
func ListHTTPSDeployKeys(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/https_keys repository repoListHTTPSKeys
	// ---
	// summary: List a repository's HTTPS deploy keys
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
	//     "$ref": "#/responses/HTTPSDeployKeyList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := asymkey_model.ListHTTPSDeployKeysOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
	}

	keys, count, err := db.FindAndCount[asymkey_model.HTTPSDeployKey](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiLink := composeHTTPSDeployKeysAPILink(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
	apiKeys := make([]*api.HTTPSDeployKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToHTTPSDeployKey(apiLink, keys[i])
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &apiKeys)
}

// GetHTTPSDeployKey get an HTTPS deploy key by id
func GetHTTPSDeployKey(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/https_keys/{id} repository repoGetHTTPSKey
	// ---
	// summary: Get a repository's HTTPS deploy key by id
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
	//   description: id of the key to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/HTTPSDeployKey"
	//   "404":
	//     "$ref": "#/responses/notFound"

	key, err := asymkey_model.GetHTTPSDeployKeyByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if asymkey_model.IsErrHTTPSDeployKeyNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if key.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return
	}

	apiLink := composeHTTPSDeployKeysAPILink(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
	ctx.JSON(http.StatusOK, convert.ToHTTPSDeployKey(apiLink, key))
}

// CreateHTTPSDeployKey create HTTPS deploy key for a repository
func CreateHTTPSDeployKey(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/https_keys repository repoCreateHTTPSKey
	// ---
	// summary: Add an HTTPS deploy key to a repository
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
	//     "$ref": "#/definitions/CreateHTTPSDeployKeyOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/HTTPSDeployKey"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateHTTPSDeployKeyOption)
	key, token, err := asymkey_model.AddHTTPSDeployKey(ctx, ctx.Repo.Repository.ID, form.Name, form.ReadOnly)
	if err != nil {
		switch {
		case asymkey_model.IsErrHTTPSDeployKeyNameAlreadyUsed(err):
			ctx.APIError(http.StatusUnprocessableEntity, "A deploy key with the same name already exists")
		case errors.Is(err, util.ErrInvalidArgument):
			ctx.APIError(http.StatusUnprocessableEntity, err)
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	log.Trace("HTTPS deploy key added (API): operator=%s repo=%s key=%s (id=%d)",
		ctx.Doer.Name, ctx.Repo.Repository.FullName(), key.Name, key.ID)

	apiLink := composeHTTPSDeployKeysAPILink(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
	apiKey := convert.ToHTTPSDeployKey(apiLink, key)
	apiKey.Token = token
	ctx.JSON(http.StatusCreated, apiKey)
}

// DeleteHTTPSDeployKey delete HTTPS deploy key for a repository
func DeleteHTTPSDeployKey(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/https_keys/{id} repository repoDeleteHTTPSKey
	// ---
	// summary: Delete an HTTPS deploy key from a repository
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
	//   description: id of the key to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	key, err := asymkey_model.GetHTTPSDeployKeyByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if asymkey_model.IsErrHTTPSDeployKeyNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if key.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return
	}

	if err := asymkey_model.DeleteHTTPSDeployKey(ctx, ctx.Repo.Repository.ID, key.ID); err != nil {
		if asymkey_model.IsErrHTTPSDeployKeyNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	log.Trace("HTTPS deploy key deleted (API): operator=%s repo=%s key=%s (id=%d)",
		ctx.Doer.Name, ctx.Repo.Repository.FullName(), key.Name, key.ID)

	ctx.Status(http.StatusNoContent)
}
