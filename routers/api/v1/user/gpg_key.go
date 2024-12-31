// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

func listGPGKeys(ctx *context.APIContext, uid int64, listOptions db.ListOptions) {
	keys, total, err := db.FindAndCount[asymkey_model.GPGKey](ctx, asymkey_model.FindGPGKeyOptions{
		ListOptions: listOptions,
		OwnerID:     uid,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListGPGKeys", err)
		return
	}

	if err := asymkey_model.GPGKeyList(keys).LoadSubKeys(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "ListGPGKeys", err)
		return
	}

	apiKeys := make([]*api.GPGKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToGPGKey(keys[i])
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &apiKeys)
}

// ListGPGKeys get the GPG key list of a user
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
	//     "$ref": "#/responses/GPGKeyList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	listGPGKeys(ctx, ctx.ContextUser.ID, utils.GetListOptions(ctx))
}

// ListMyGPGKeys get the GPG key list of the authenticated user
func ListMyGPGKeys(ctx *context.APIContext) {
	// swagger:operation GET /user/gpg_keys user userCurrentListGPGKeys
	// ---
	// summary: List the authenticated user's GPG keys
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/GPGKeyList"

	listGPGKeys(ctx, ctx.Doer.ID, utils.GetListOptions(ctx))
}

// GetGPGKey get the GPG key based on a id
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

	key, err := asymkey_model.GetGPGKeyForUserByID(ctx, ctx.Doer.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if asymkey_model.IsErrGPGKeyNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetGPGKeyByID", err)
		}
		return
	}
	if err := key.LoadSubKeys(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadSubKeys", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToGPGKey(key))
}

// CreateUserGPGKey creates new GPG key to given user by ID.
func CreateUserGPGKey(ctx *context.APIContext, form api.CreateGPGKeyOption, uid int64) {
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageGPGKeys) {
		ctx.NotFound("Not Found", fmt.Errorf("gpg keys setting is not allowed to be visited"))
		return
	}

	token := asymkey_model.VerificationToken(ctx.Doer, 1)
	lastToken := asymkey_model.VerificationToken(ctx.Doer, 0)

	keys, err := asymkey_model.AddGPGKey(ctx, uid, form.ArmoredKey, token, form.Signature)
	if err != nil && asymkey_model.IsErrGPGInvalidTokenSignature(err) {
		keys, err = asymkey_model.AddGPGKey(ctx, uid, form.ArmoredKey, lastToken, form.Signature)
	}
	if err != nil {
		HandleAddGPGKeyError(ctx, err, token)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToGPGKey(keys[0]))
}

// GetVerificationToken returns the current token to be signed for this user
func GetVerificationToken(ctx *context.APIContext) {
	// swagger:operation GET /user/gpg_key_token user getVerificationToken
	// ---
	// summary: Get a Token to verify
	// produces:
	// - text/plain
	// parameters:
	// responses:
	//   "200":
	//     "$ref": "#/responses/string"
	//   "404":
	//     "$ref": "#/responses/notFound"

	token := asymkey_model.VerificationToken(ctx.Doer, 1)
	ctx.PlainText(http.StatusOK, token)
}

// VerifyUserGPGKey creates new GPG key to given user by ID.
func VerifyUserGPGKey(ctx *context.APIContext) {
	// swagger:operation POST /user/gpg_key_verify user userVerifyGPGKey
	// ---
	// summary: Verify a GPG key
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// responses:
	//   "201":
	//     "$ref": "#/responses/GPGKey"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.VerifyGPGKeyOption)
	token := asymkey_model.VerificationToken(ctx.Doer, 1)
	lastToken := asymkey_model.VerificationToken(ctx.Doer, 0)

	form.KeyID = strings.TrimLeft(form.KeyID, "0")
	if form.KeyID == "" {
		ctx.NotFound()
		return
	}

	_, err := asymkey_model.VerifyGPGKey(ctx, ctx.Doer.ID, form.KeyID, token, form.Signature)
	if err != nil && asymkey_model.IsErrGPGInvalidTokenSignature(err) {
		_, err = asymkey_model.VerifyGPGKey(ctx, ctx.Doer.ID, form.KeyID, lastToken, form.Signature)
	}

	if err != nil {
		if asymkey_model.IsErrGPGInvalidTokenSignature(err) {
			ctx.Error(http.StatusUnprocessableEntity, "GPGInvalidSignature", fmt.Sprintf("The provided GPG key, signature and token do not match or token is out of date. Provide a valid signature for the token: %s", token))
			return
		}
		ctx.Error(http.StatusInternalServerError, "VerifyUserGPGKey", err)
	}

	keys, err := db.Find[asymkey_model.GPGKey](ctx, asymkey_model.FindGPGKeyOptions{
		KeyID:          form.KeyID,
		IncludeSubKeys: true,
	})
	if err != nil {
		if asymkey_model.IsErrGPGKeyNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetGPGKeysByKeyID", err)
		}
		return
	}
	ctx.JSON(http.StatusOK, convert.ToGPGKey(keys[0]))
}

// swagger:parameters userCurrentPostGPGKey
type swaggerUserCurrentPostGPGKey struct {
	// in:body
	Form api.CreateGPGKeyOption
}

// CreateGPGKey create a GPG key belonging to the authenticated user
func CreateGPGKey(ctx *context.APIContext) {
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateGPGKeyOption)
	CreateUserGPGKey(ctx, *form, ctx.Doer.ID)
}

// DeleteGPGKey remove a GPG key belonging to the authenticated user
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageGPGKeys) {
		ctx.NotFound("Not Found", fmt.Errorf("gpg keys setting is not allowed to be visited"))
		return
	}

	if err := asymkey_model.DeleteGPGKey(ctx, ctx.Doer, ctx.PathParamInt64("id")); err != nil {
		if asymkey_model.IsErrGPGKeyAccessDenied(err) {
			ctx.Error(http.StatusForbidden, "", "You do not have access to this key")
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteGPGKey", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// HandleAddGPGKeyError handle add GPGKey error
func HandleAddGPGKeyError(ctx *context.APIContext, err error, token string) {
	switch {
	case asymkey_model.IsErrGPGKeyAccessDenied(err):
		ctx.Error(http.StatusUnprocessableEntity, "GPGKeyAccessDenied", "You do not have access to this GPG key")
	case asymkey_model.IsErrGPGKeyIDAlreadyUsed(err):
		ctx.Error(http.StatusUnprocessableEntity, "GPGKeyIDAlreadyUsed", "A key with the same id already exists")
	case asymkey_model.IsErrGPGKeyParsing(err):
		ctx.Error(http.StatusUnprocessableEntity, "GPGKeyParsing", err)
	case asymkey_model.IsErrGPGNoEmailFound(err):
		ctx.Error(http.StatusNotFound, "GPGNoEmailFound", fmt.Sprintf("None of the emails attached to the GPG key could be found. It may still be added if you provide a valid signature for the token: %s", token))
	case asymkey_model.IsErrGPGInvalidTokenSignature(err):
		ctx.Error(http.StatusUnprocessableEntity, "GPGInvalidSignature", fmt.Sprintf("The provided GPG key, signature and token do not match or token is out of date. Provide a valid signature for the token: %s", token))
	default:
		ctx.Error(http.StatusInternalServerError, "AddGPGKey", err)
	}
}
