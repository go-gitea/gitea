// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	secret_model "code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/shared/actions"
)

// create or update one secret of the user scope
func CreateOrUpdateSecret(ctx *context.APIContext) {
	// swagger:operation PUT /user/actions/secrets/{secretname} user updateUserSecret
	// ---
	// summary: Create or Update a secret value in a user scope
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateOrUpdateSecretOption"
	// responses:
	//   "201":
	//     description: response when creating a secret
	//   "204":
	//     description: response when updating a secret
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	secretName := ctx.Params(":secretname")
	if err := actions.NameRegexMatch(secretName); err != nil {
		ctx.Error(http.StatusBadRequest, "CreateOrUpdateSecret", err)
		return
	}
	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)
	isCreated, err := secret_model.CreateOrUpdateSecret(ctx, ctx.Doer.ID, 0, secretName, opt.Data)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateOrUpdateSecret", err)
		return
	}
	if isCreated {
		ctx.Status(http.StatusCreated)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteSecret delete one secret of the user scope
func DeleteSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /user/actions/secrets/{secretname} user deleteUserSecret
	// ---
	// summary: Delete a secret in a user scope
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: delete one secret of the user
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	secretName := ctx.Params(":secretname")
	if err := actions.NameRegexMatch(secretName); err != nil {
		ctx.Error(http.StatusBadRequest, "DeleteSecret", err)
		return
	}
	err := secret_model.DeleteSecret(
		ctx, ctx.Doer.ID, 0, secretName,
	)
	if secret_model.IsErrSecretNotFound(err) {
		ctx.NotFound(err)
		return
	}
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteSecret", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
