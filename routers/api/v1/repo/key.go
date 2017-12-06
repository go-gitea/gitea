// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/convert"
)

func composeDeployKeysAPILink(repoPath string) string {
	return setting.AppURL + "api/v1/repos/" + repoPath + "/keys/"
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/DeployKeyList"
	keys, err := models.ListDeployKeys(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Error(500, "ListDeployKeys", err)
		return
	}

	apiLink := composeDeployKeysAPILink(ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name)
	apiKeys := make([]*api.DeployKey, len(keys))
	for i := range keys {
		if err = keys[i].GetContent(); err != nil {
			ctx.Error(500, "GetContent", err)
			return
		}
		apiKeys[i] = convert.ToDeployKey(apiLink, keys[i])
	}

	ctx.JSON(200, &apiKeys)
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
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/DeployKey"
	key, err := models.GetDeployKeyByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrDeployKeyNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetDeployKeyByID", err)
		}
		return
	}

	if err = key.GetContent(); err != nil {
		ctx.Error(500, "GetContent", err)
		return
	}

	apiLink := composeDeployKeysAPILink(ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name)
	ctx.JSON(200, convert.ToDeployKey(apiLink, key))
}

// HandleCheckKeyStringError handle check key error
func HandleCheckKeyStringError(ctx *context.APIContext, err error) {
	if models.IsErrSSHDisabled(err) {
		ctx.Error(422, "", "SSH is disabled")
	} else if models.IsErrKeyUnableVerify(err) {
		ctx.Error(422, "", "Unable to verify key content")
	} else {
		ctx.Error(422, "", fmt.Errorf("Invalid key content: %v", err))
	}
}

// HandleAddKeyError handle add key error
func HandleAddKeyError(ctx *context.APIContext, err error) {
	switch {
	case models.IsErrKeyAlreadyExist(err):
		ctx.Error(422, "", "Key content has been used as non-deploy key")
	case models.IsErrKeyNameAlreadyUsed(err):
		ctx.Error(422, "", "Key title has been used")
	default:
		ctx.Error(500, "AddKey", err)
	}
}

// CreateDeployKey create deploy key for a repository
func CreateDeployKey(ctx *context.APIContext, form api.CreateKeyOption) {
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
	content, err := models.CheckPublicKeyString(form.Key)
	if err != nil {
		HandleCheckKeyStringError(ctx, err)
		return
	}

	key, err := models.AddDeployKey(ctx.Repo.Repository.ID, form.Title, content)
	if err != nil {
		HandleAddKeyError(ctx, err)
		return
	}

	key.Content = content
	apiLink := composeDeployKeysAPILink(ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name)
	ctx.JSON(201, convert.ToDeployKey(apiLink, key))
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
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	if err := models.DeleteDeployKey(ctx.User, ctx.ParamsInt64(":id")); err != nil {
		if models.IsErrKeyAccessDenied(err) {
			ctx.Error(403, "", "You do not have access to this key")
		} else {
			ctx.Error(500, "DeleteDeployKey", err)
		}
		return
	}

	ctx.Status(204)
}
