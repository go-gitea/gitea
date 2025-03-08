// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
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

	_, created, err := secret_service.CreateOrUpdateSecret(ctx, ctx.Doer.ID, 0, ctx.PathParam("secretname"), opt.Data)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
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

	err := secret_service.DeleteSecretByName(ctx, ctx.Doer.ID, 0, ctx.PathParam("secretname"))
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateVariable create a user-level variable
func CreateVariable(ctx *context.APIContext) {
	// swagger:operation POST /user/actions/variables/{variablename} user createUserVariable
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
	//     "$ref": "#/definitions/CreateVariableOption"
	// responses:
	//   "201":
	//     description: response when creating a variable
	//   "204":
	//     description: response when creating a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.CreateVariableOption)

	ownerID := ctx.Doer.ID
	variableName := ctx.PathParam("variablename")

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		OwnerID: ownerID,
		Name:    variableName,
	})
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.APIErrorInternal(err)
		return
	}
	if v != nil && v.ID > 0 {
		ctx.APIError(http.StatusConflict, util.NewAlreadyExistErrorf("variable name %s already exists", variableName))
		return
	}

	if _, err := actions_service.CreateVariable(ctx, ownerID, 0, variableName, opt.Value); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// UpdateVariable update a user-level variable which is created by current doer
func UpdateVariable(ctx *context.APIContext) {
	// swagger:operation PUT /user/actions/variables/{variablename} user updateUserVariable
	// ---
	// summary: Update a user-level variable which is created by current doer
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
	//     "$ref": "#/definitions/UpdateVariableOption"
	// responses:
	//   "201":
	//     description: response when updating a variable
	//   "204":
	//     description: response when updating a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.UpdateVariableOption)

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		OwnerID: ctx.Doer.ID,
		Name:    ctx.PathParam("variablename"),
	})
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if opt.Name == "" {
		opt.Name = ctx.PathParam("variablename")
	}

	v.Name = opt.Name
	v.Data = opt.Value

	if _, err := actions_service.UpdateVariableNameData(ctx, v); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteVariable delete a user-level variable which is created by current doer
func DeleteVariable(ctx *context.APIContext) {
	// swagger:operation DELETE /user/actions/variables/{variablename} user deleteUserVariable
	// ---
	// summary: Delete a user-level variable which is created by current doer
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     description: response when deleting a variable
	//   "204":
	//     description: response when deleting a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := actions_service.DeleteVariableByName(ctx, ctx.Doer.ID, 0, ctx.PathParam("variablename")); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// GetVariable get a user-level variable which is created by current doer
func GetVariable(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/variables/{variablename} user getUserVariable
	// ---
	// summary: Get a user-level variable which is created by current doer
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//			"$ref": "#/responses/ActionVariable"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		OwnerID: ctx.Doer.ID,
		Name:    ctx.PathParam("variablename"),
	})
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	variable := &api.ActionVariable{
		OwnerID: v.OwnerID,
		RepoID:  v.RepoID,
		Name:    v.Name,
		Data:    v.Data,
	}

	ctx.JSON(http.StatusOK, variable)
}

// ListVariables list user-level variables
func ListVariables(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/variables user getUserVariablesList
	// ---
	// summary: Get the user-level list of variables which is created by current doer
	// produces:
	// - application/json
	// parameters:
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
	//			"$ref": "#/responses/VariableList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	vars, count, err := db.FindAndCount[actions_model.ActionVariable](ctx, &actions_model.FindVariablesOpts{
		OwnerID:     ctx.Doer.ID,
		ListOptions: utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	variables := make([]*api.ActionVariable, len(vars))
	for i, v := range vars {
		variables[i] = &api.ActionVariable{
			OwnerID: v.OwnerID,
			RepoID:  v.RepoID,
			Name:    v.Name,
			Data:    v.Data,
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, variables)
}
