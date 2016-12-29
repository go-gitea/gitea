// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// ListCollaborators list a repository's collaborators
func ListCollaborators(ctx *context.APIContext) {
	if !ctx.Repo.IsWriter() {
		ctx.Error(403, "", "User does not have push access")
		return
	}
	collaborators, err := ctx.Repo.Repository.GetCollaborators()
	if err != nil {
		ctx.Error(500, "ListCollaborators", err)
		return
	}
	users := make([]*api.User, len(collaborators))
	for i, collaborator := range collaborators {
		users[i] = collaborator.APIFormat()
	}
	ctx.JSON(200, users)
}

// IsCollaborator check if a user is a collaborator of a repository
func IsCollaborator(ctx *context.APIContext) {
	if !ctx.Repo.IsWriter() {
		ctx.Error(403, "", "User does not have push access")
		return
	}
	user, err := models.GetUserByName(ctx.Params(":collaborator"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return
	}
	isColab, err := ctx.Repo.Repository.IsCollaborator(user.ID)
	if err != nil {
		ctx.Error(500, "IsCollaborator", err)
		return
	}
	if isColab {
		ctx.Status(204)
	} else {
		ctx.Status(404)
	}
}

// AddCollaborator add a collaborator of a repository
func AddCollaborator(ctx *context.APIContext, form api.AddCollaboratorOption) {
	if !ctx.Repo.IsWriter() {
		ctx.Error(403, "", "User does not have push access")
		return
	}
	collaborator, err := models.GetUserByName(ctx.Params(":collaborator"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return
	}

	if err := ctx.Repo.Repository.AddCollaborator(collaborator); err != nil {
		ctx.Error(500, "AddCollaborator", err)
		return
	}

	if form.Permission != nil {
		if err := ctx.Repo.Repository.ChangeCollaborationAccessMode(collaborator.ID, models.ParseAccessMode(*form.Permission)); err != nil {
			ctx.Error(500, "ChangeCollaborationAccessMode", err)
			return
		}
	}

	ctx.Status(204)
}

// DeleteCollaborator delete a collaborator from a repository
func DeleteCollaborator(ctx *context.APIContext) {
	if !ctx.Repo.IsWriter() {
		ctx.Error(403, "", "User does not have push access")
		return
	}

	collaborator, err := models.GetUserByName(ctx.Params(":collaborator"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return
	}

	if err := ctx.Repo.Repository.DeleteCollaboration(collaborator.ID); err != nil {
		ctx.Error(500, "DeleteCollaboration", err)
		return
	}
	ctx.Status(204)
}
