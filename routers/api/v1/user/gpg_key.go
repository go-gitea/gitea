// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
)

func listGPGKeys(ctx *context.APIContext, uid int64) {
	keys, err := models.ListGPGKeys(uid)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListGPGKeys", err)
		return
	}

	apiKeys := make([]*api.GPGKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToGPGKey(keys[i])
	}

	ctx.JSON(http.StatusOK, &apiKeys)
}

//ListGPGKeys get the GPG key list of a user
func ListGPGKeys(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/gpg_keys user userListGPGKeys
	// ---
	// summary: List the given user's GPG keys
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/GPGKeyList"

	user := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listGPGKeys(ctx, user.ID)
}

//ListMyGPGKeys get the GPG key list of the authenticated user
func ListMyGPGKeys(ctx *context.APIContext) {
	// swagger:operation GET /user/gpg_keys user userCurrentListGPGKeys
	// ---
	// summary: List the authenticated user's GPG keys
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/GPGKeyList"

	listGPGKeys(ctx, ctx.User.ID)
}

//GetGPGKey get the GPG key based on a id
func GetGPGKey(ctx *context.APIContext) {
	// swagger:operation GET /user/gpg_keys/{id} user userCurrentGetGPGKey
	// ---
	// summary: Get a GPG key
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of key to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/GPGKey"
	//   "404":
	//     "$ref": "#/responses/notFound"

	key, err := models.GetGPGKeyByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrGPGKeyNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetGPGKeyByID", err)
		}
		return
	}
	ctx.JSON(http.StatusOK, convert.ToGPGKey(key))
}

// CreateUserGPGKey creates new GPG key to given user by ID.
func CreateUserGPGKey(ctx *context.APIContext, form api.CreateGPGKeyOption, uid int64) {
	key, err := models.AddGPGKey(uid, form.ArmoredKey)
	if err != nil {
		HandleAddGPGKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToGPGKey(key))
}

// swagger:parameters userCurrentPostGPGKey
type swaggerUserCurrentPostGPGKey struct {
	// in:body
	Form api.CreateGPGKeyOption
}

//CreateGPGKey create a GPG key belonging to the authenticated user
func CreateGPGKey(ctx *context.APIContext, form api.CreateGPGKeyOption) {
	// swagger:operation POST /user/gpg_keys user userCurrentPostGPGKey
	// ---
	// summary: Create a GPG key
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// responses:
	//   "201":
	//     "$ref": "#/responses/GPGKey"
	//   "422":
	//     "$ref": "#/responses/validationError"

	CreateUserGPGKey(ctx, form, ctx.User.ID)
}

//DeleteGPGKey remove a GPG key belonging to the authenticated user
func DeleteGPGKey(ctx *context.APIContext) {
	// swagger:operation DELETE /user/gpg_keys/{id} user userCurrentDeleteGPGKey
	// ---
	// summary: Remove a GPG key
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of key to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	if err := models.DeleteGPGKey(ctx.User, ctx.ParamsInt64(":id")); err != nil {
		if models.IsErrGPGKeyAccessDenied(err) {
			ctx.Error(http.StatusForbidden, "", "You do not have access to this key")
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteGPGKey", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// HandleAddGPGKeyError handle add GPGKey error
func HandleAddGPGKeyError(ctx *context.APIContext, err error) {
	switch {
	case models.IsErrGPGKeyAccessDenied(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "You do not have access to this GPG key")
	case models.IsErrGPGKeyIDAlreadyUsed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "A key with the same id already exists")
	default:
		ctx.Error(http.StatusInternalServerError, "AddGPGKey", err)
	}
}
