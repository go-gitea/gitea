// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/convert"
)

func composePublicGPGKeysAPILink() string {
	return setting.AppURL + "api/v1/user/gpg_keys/"
}

func listGPGKeys(ctx *context.APIContext, uid int64) {
	keys, err := models.ListGPGKeys(uid)
	if err != nil {
		ctx.Error(500, "ListGPGKeys", err)
		return
	}

	apiKeys := make([]*api.GPGKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToGPGKey(keys[i])
	}

	ctx.JSON(200, &apiKeys)
}

//ListGPGKeys get the GPG key list of a user
func ListGPGKeys(ctx *context.APIContext) {
	// swagger:route GET /users/{username}/gpg_keys userListGPGKeys
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: GPGKeyList
	//       500: error

	user := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listGPGKeys(ctx, user.ID)
}

//ListMyGPGKeys get the GPG key list of the logged user
func ListMyGPGKeys(ctx *context.APIContext) {
	// swagger:route GET /user/gpg_keys userCurrentListGPGKeys
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: GPGKeyList
	//       500: error

	listGPGKeys(ctx, ctx.User.ID)
}

//GetGPGKey get the GPG key based on a id
func GetGPGKey(ctx *context.APIContext) {
	// swagger:route GET /user/gpg_keys/{id} userCurrentGetGPGKey
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: GPGKey
	//       404: notFound
	//       500: error

	key, err := models.GetGPGKeyByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrGPGKeyNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetGPGKeyByID", err)
		}
		return
	}
	ctx.JSON(200, convert.ToGPGKey(key))
}

// CreateUserGPGKey creates new GPG key to given user by ID.
func CreateUserGPGKey(ctx *context.APIContext, form api.CreateGPGKeyOption, uid int64) {
	key, err := models.AddGPGKey(uid, form.ArmoredKey)
	if err != nil {
		HandleAddGPGKeyError(ctx, err)
		return
	}
	ctx.JSON(201, convert.ToGPGKey(key))
}

//CreateGPGKey associate a GPG key to the current user
func CreateGPGKey(ctx *context.APIContext, form api.CreateGPGKeyOption) {
	// swagger:route POST /user/gpg_keys userCurrentPostGPGKey
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       201: GPGKey
	//       422: validationError
	//       500: error

	CreateUserGPGKey(ctx, form, ctx.User.ID)
}

//DeleteGPGKey remove a GPG key associated to the current user
func DeleteGPGKey(ctx *context.APIContext) {
	// swagger:route DELETE /user/gpg_keys/{id} userCurrentDeleteGPGKey
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       204: empty
	//       403: forbidden
	//       500: error

	if err := models.DeleteGPGKey(ctx.User, ctx.ParamsInt64(":id")); err != nil {
		if models.IsErrGPGKeyAccessDenied(err) {
			ctx.Error(403, "", "You do not have access to this key")
		} else {
			ctx.Error(500, "DeleteGPGKey", err)
		}
		return
	}

	ctx.Status(204)
}

// HandleAddGPGKeyError handle add GPGKey error
func HandleAddGPGKeyError(ctx *context.APIContext, err error) {
	switch {
	case models.IsErrGPGKeyAccessDenied(err):
		ctx.Error(422, "", "You do not have access to this gpg key")
	case models.IsErrGPGKeyIDAlreadyUsed(err):
		ctx.Error(422, "", "A key with the same keyid is already in database")
	default:
		ctx.Error(500, "AddGPGKey", err)
	}
}
