// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
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
	opts := &secret.FindSecretsOptions{
		OwnerID:     ctx.Org.Organization.ID,
		ListOptions: utils.GetListOptions(ctx),
	}

	count, err := secret.CountSecrets(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	secrets, err := secret.FindSecrets(ctx, *opts)
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
