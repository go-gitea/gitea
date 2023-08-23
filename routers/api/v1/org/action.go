// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	secret_model "code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/routers/web/shared/actions"
	"code.gitea.io/gitea/services/convert"
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

	listActionsSecrets(ctx)
}

// listActionsSecrets list an organization's actions secrets
func listActionsSecrets(ctx *context.APIContext) {
	opts := &secret_model.FindSecretsOptions{
		OwnerID:     ctx.Org.Organization.ID,
		ListOptions: utils.GetListOptions(ctx),
	}

	count, err := secret_model.CountSecrets(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	secrets, err := secret_model.FindSecrets(ctx, *opts)
	if err != nil {
		ctx.InternalServerError(err)
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

// CreateOrgSecret create one secret of the organization
func CreateOrgSecret(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/actions/secrets organization createOrgSecret
	// ---
	// summary: Create a secret in an organization
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateSecretOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Secret"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	opt := web.GetForm(ctx).(*api.CreateSecretOption)
	s, err := secret_model.InsertEncryptedSecret(
		ctx, ctx.Org.Organization.ID, 0, opt.Name, actions.ReserveLineBreakForTextarea(opt.Data),
	)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "InsertEncryptedSecret", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToSecret(s))
}
