// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/db"
	secret_model "code.gitea.io/gitea/models/secret"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	secret_service "code.gitea.io/gitea/services/secrets"
)

// ListActionsSecrets list an organization's actions secrets
func ListActionsSecrets(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/secrets organization orgListActionsSecrets
	// ---
	// summary: List an organization's actions secrets
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//     "$ref": "#/responses/SecretList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := &secret_model.FindSecretsOptions{
		OwnerID:     ctx.Org.Organization.ID,
		ListOptions: utils.GetListOptions(ctx),
	}

	secrets, count, err := db.FindAndCount[secret_model.Secret](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiSecrets := make([]*api.Secret, len(secrets))
	for k, v := range secrets {
		apiSecrets[k] = &api.Secret{
			Name:    v.Name,
			Created: v.CreatedUnix.AsTime(),
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiSecrets)
}

// CreateOrUpdateSecret create or update one secret in an organization
func CreateOrUpdateSecret(ctx *context.APIContext) {
	// swagger:operation PUT /orgs/{org}/actions/secrets/{secretname} organization updateOrgSecret
	// ---
	// summary: Create or Update a secret value in an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of organization
	//   type: string
	//   required: true
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
	//     description: secret created
	//   "204":
	//     description: secret updated
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)

	_, created, err := secret_service.CreateOrUpdateSecret(ctx, ctx.Org.Organization.ID, 0, ctx.PathParam("secretname"), opt.Data, opt.Description)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIError(http.StatusInternalServerError, err)
		}
		return
	}

	if created {
		ctx.Status(http.StatusCreated)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// DeleteSecret delete one secret in an organization
func DeleteSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/actions/secrets/{secretname} organization deleteOrgSecret
	// ---
	// summary: Delete a secret in an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of organization
	//   type: string
	//   required: true
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: secret deleted
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := secret_service.DeleteSecretByName(ctx, ctx.Org.Organization.ID, 0, ctx.PathParam("secretname"))
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIError(http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
