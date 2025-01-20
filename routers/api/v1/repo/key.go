// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repo

import (
	stdCtx "context"
	"fmt"
	"net/http"
	"net/url"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// appendPrivateInformation appends the owner and key type information to api.PublicKey
func appendPrivateInformation(ctx stdCtx.Context, apiKey *api.DeployKey, key *asymkey_model.DeployKey, repository *repo_model.Repository) (*api.DeployKey, error) {
	apiKey.ReadOnly = key.Mode == perm.AccessModeRead
	if repository.ID == key.RepoID {
		apiKey.Repository = convert.ToRepo(ctx, repository, access_model.Permission{AccessMode: key.Mode})
	} else {
		repo, err := repo_model.GetRepositoryByID(ctx, key.RepoID)
		if err != nil {
			return apiKey, err
		}
		apiKey.Repository = convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: key.Mode})
	}
	return apiKey, nil
}

func composeDeployKeysAPILink(owner, name string) string {
	return setting.AppURL + "api/v1/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/keys/"
}

// ListDeployKeys list all the deploy keys of a repository
func ListDeployKeys(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/keys repository repoListKeys
	// ---
	// summary: List a repository's keys
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
	// - name: key_id
	//   in: query
	//   description: the key_id to search for
	//   type: integer
	// - name: fingerprint
	//   in: query
	//   description: fingerprint of the key
	//   type: string
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
	//     "$ref": "#/responses/DeployKeyList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := asymkey_model.ListDeployKeysOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
		KeyID:       ctx.FormInt64("key_id"),
		Fingerprint: ctx.FormString("fingerprint"),
	}

	keys, count, err := db.FindAndCount[asymkey_model.DeployKey](ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiLink := composeDeployKeysAPILink(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
	apiKeys := make([]*api.DeployKey, len(keys))
	for i := range keys {
		if err := keys[i].GetContent(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, "GetContent", err)
			return
		}
		apiKeys[i] = convert.ToDeployKey(apiLink, keys[i])
		if ctx.Doer.IsAdmin || ((ctx.Repo.Repository.ID == keys[i].RepoID) && (ctx.Doer.ID == ctx.Repo.Owner.ID)) {
			apiKeys[i], _ = appendPrivateInformation(ctx, apiKeys[i], keys[i], ctx.Repo.Repository)
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &apiKeys)
}

// GetDeployKey get a deploy key by id
func GetDeployKey(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/keys/{id} repository repoGetKey
	// ---
	// summary: Get a repository's key by id
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
	//     "$ref": "#/responses/DeployKey"
	//   "404":
	//     "$ref": "#/responses/notFound"

	key, err := asymkey_model.GetDeployKeyByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if asymkey_model.IsErrDeployKeyNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetDeployKeyByID", err)
		}
		return
	}

	// this check make it more consistent
	if key.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return
	}

	if err = key.GetContent(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetContent", err)
		return
	}

	apiLink := composeDeployKeysAPILink(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
	apiKey := convert.ToDeployKey(apiLink, key)
	if ctx.Doer.IsAdmin || ((ctx.Repo.Repository.ID == key.RepoID) && (ctx.Doer.ID == ctx.Repo.Owner.ID)) {
		apiKey, _ = appendPrivateInformation(ctx, apiKey, key, ctx.Repo.Repository)
	}
	ctx.JSON(http.StatusOK, apiKey)
}

// HandleCheckKeyStringError handle check key error
func HandleCheckKeyStringError(ctx *context.APIContext, err error) {
	if db.IsErrSSHDisabled(err) {
		ctx.Error(http.StatusUnprocessableEntity, "", "SSH is disabled")
	} else if asymkey_model.IsErrKeyUnableVerify(err) {
		ctx.Error(http.StatusUnprocessableEntity, "", "Unable to verify key content")
	} else {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("Invalid key content: %w", err))
	}
}

// HandleAddKeyError handle add key error
func HandleAddKeyError(ctx *context.APIContext, err error) {
	switch {
	case asymkey_model.IsErrDeployKeyAlreadyExist(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "This key has already been added to this repository")
	case asymkey_model.IsErrKeyAlreadyExist(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "Key content has been used as non-deploy key")
	case asymkey_model.IsErrKeyNameAlreadyUsed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "Key title has been used")
	case asymkey_model.IsErrDeployKeyNameAlreadyUsed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "A key with the same name already exists")
	default:
		ctx.Error(http.StatusInternalServerError, "AddKey", err)
	}
}

// CreateDeployKey create deploy key for a repository
func CreateDeployKey(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/keys repository repoCreateKey
	// ---
	// summary: Add a key to a repository
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
	//     "$ref": "#/definitions/CreateKeyOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/DeployKey"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateKeyOption)
	content, err := asymkey_model.CheckPublicKeyString(form.Key)
	if err != nil {
		HandleCheckKeyStringError(ctx, err)
		return
	}

	key, err := asymkey_model.AddDeployKey(ctx, ctx.Repo.Repository.ID, form.Title, content, form.ReadOnly)
	if err != nil {
		HandleAddKeyError(ctx, err)
		return
	}

	key.Content = content
	apiLink := composeDeployKeysAPILink(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
	ctx.JSON(http.StatusCreated, convert.ToDeployKey(apiLink, key))
}

// DeleteDeploykey delete deploy key for a repository
func DeleteDeploykey(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/keys/{id} repository repoDeleteKey
	// ---
	// summary: Delete a key from a repository
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

	if err := asymkey_service.DeleteDeployKey(ctx, ctx.Repo.Repository, ctx.PathParamInt64("id")); err != nil {
		if asymkey_model.IsErrKeyAccessDenied(err) {
			ctx.Error(http.StatusForbidden, "", "You do not have access to this key")
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteDeployKey", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
