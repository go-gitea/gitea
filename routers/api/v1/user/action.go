// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"fmt"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/actions"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/context"
	secret_service "code.gitea.io/gitea/services/secrets"
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

	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)

	_, created, err := secret_service.CreateOrUpdateSecret(ctx, ctx.Doer.ID, 0, ctx.Params("secretname"), opt.Data)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "CreateOrUpdateSecret", err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "CreateOrUpdateSecret", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateOrUpdateSecret", err)
		}
		return
	}

	if created {
		ctx.Status(http.StatusCreated)
	} else {
		ctx.Status(http.StatusNoContent)
	}
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

	err := secret_service.DeleteSecretByName(ctx, ctx.Doer.ID, 0, ctx.Params("secretname"))
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "DeleteSecret", err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DeleteSecret", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteSecret", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateVariable create a user-level variable
func CreateVariable(ctx *context.APIContext) {
	// swagger:operation PUT /user/actions/variables/{variablename} user createUserVariable
	// ---
	// summary: Create a user-level variable
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateVariable"
	// responses:
	//   "201":
	//     description: response when creating a variable
	//   "204":
	//     description: response when updating a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.CreateVariableOption)

	if _, err := actions_service.CreateVariable(ctx, ctx.Doer.ID, 0, ctx.Params("variablename"), opt.Value); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "CreateVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateVariable", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// UpdateVariable update a user-level variable
func UpdateVariable(ctx *context.APIContext) {

	opt := web.GetForm(ctx).(*api.UpdateVariableOption)

	v, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		OwnerID: ctx.Doer.ID,
		Name:    ctx.Params("variablename"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindVariable", err)
		return
	}
	if len(v) == 0 {
		ctx.Error(http.StatusNotFound, "FindVariable", fmt.Errorf("varibale not found"))
		return
	}

	if opt.Name == "" {
		opt.Name = ctx.Params("variablename")
	}
	if _, err := actions.UpdateVariable(ctx, v[0].ID, opt.Name, opt.Value); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "UpdateVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "UpdateVariable", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteVaribale delete a user-level variable
func DeleteVaribale(ctx *context.APIContext) {

}
