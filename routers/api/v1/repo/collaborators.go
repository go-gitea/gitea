// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
)

// ListCollaborators list a repository's collaborators
func ListCollaborators(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/collaborators repository repoListCollaborators
	// ---
	// summary: List a repository's collaborators
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	collaborators, err := ctx.Repo.Repository.GetCollaborators()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListCollaborators", err)
		return
	}
	users := make([]*api.User, len(collaborators))
	for i, collaborator := range collaborators {
		users[i] = convert.ToUser(collaborator.User, ctx.IsSigned, ctx.User != nil && ctx.User.IsAdmin)
	}
	ctx.JSON(http.StatusOK, users)
}

// IsCollaborator check if a user is a collaborator of a repository
func IsCollaborator(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/collaborators/{collaborator} repository repoCheckCollaborator
	// ---
	// summary: Check if a user is a collaborator of a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: collaborator
	//   in: path
	//   description: username of the collaborator
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	user, err := models.GetUserByName(ctx.Params(":collaborator"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
		}
		return
	}
	isColab, err := ctx.Repo.Repository.IsCollaborator(user.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsCollaborator", err)
		return
	}
	if isColab {
		ctx.Status(http.StatusNoContent)
	} else {
		ctx.NotFound()
	}
}

// AddCollaborator add a collaborator to a repository
func AddCollaborator(ctx *context.APIContext, form api.AddCollaboratorOption) {
	// swagger:operation PUT /repos/{owner}/{repo}/collaborators/{collaborator} repository repoAddCollaborator
	// ---
	// summary: Add a collaborator to a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: collaborator
	//   in: path
	//   description: username of the collaborator to add
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/AddCollaboratorOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "422":
	//     "$ref": "#/responses/validationError"

	collaborator, err := models.GetUserByName(ctx.Params(":collaborator"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
		}
		return
	}

	if !collaborator.IsActive {
		ctx.Error(http.StatusInternalServerError, "InactiveCollaborator", errors.New("collaborator's account is inactive"))
		return
	}

	if err := ctx.Repo.Repository.AddCollaborator(collaborator); err != nil {
		ctx.Error(http.StatusInternalServerError, "AddCollaborator", err)
		return
	}

	if form.Permission != nil {
		if err := ctx.Repo.Repository.ChangeCollaborationAccessMode(collaborator.ID, models.ParseAccessMode(*form.Permission)); err != nil {
			ctx.Error(http.StatusInternalServerError, "ChangeCollaborationAccessMode", err)
			return
		}
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteCollaborator delete a collaborator from a repository
func DeleteCollaborator(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/collaborators/{collaborator} repository repoDeleteCollaborator
	// ---
	// summary: Delete a collaborator from a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: collaborator
	//   in: path
	//   description: username of the collaborator to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "422":
	//     "$ref": "#/responses/validationError"

	collaborator, err := models.GetUserByName(ctx.Params(":collaborator"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
		}
		return
	}

	if err := ctx.Repo.Repository.DeleteCollaboration(collaborator.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteCollaboration", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
