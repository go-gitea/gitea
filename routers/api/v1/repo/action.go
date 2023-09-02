// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	secret_model "code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/shared/actions"
)

// create or update one secret of the repository
func CreateOrUpdateSecret(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/secrets/{secretname} repository updateRepoSecret
	// ---
	// summary: Create or Update a secret value in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
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
	//     description: response when creating a secret
	//   "204":
	//     description: response when updating a secret
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	owner := ctx.Repo.Owner
	repo := ctx.Repo.Repository

	secretName := ctx.Params(":secretname")
	if err := actions.NameRegexMatch(secretName); err != nil {
		ctx.Error(http.StatusBadRequest, "CreateOrUpdateSecret", err)
		return
	}
	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)
	isCreated, err := secret_model.CreateOrUpdateSecret(ctx, owner.ID, repo.ID, secretName, opt.Data)
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

// DeleteSecret delete one secret of the repository
func DeleteSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/secrets/{secretname} repository deleteRepoSecret
	// ---
	// summary: Delete a secret in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: delete one secret of the organization
	//   "403":
	//     "$ref": "#/responses/forbidden"

	owner := ctx.Repo.Owner
	repo := ctx.Repo.Repository

	secretName := ctx.Params(":secretname")
	if err := actions.NameRegexMatch(secretName); err != nil {
		ctx.Error(http.StatusBadRequest, "DeleteSecret", err)
		return
	}
	err := secret_model.DeleteSecret(
		ctx, owner.ID, repo.ID, secretName,
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
