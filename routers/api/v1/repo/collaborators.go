// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "github.com/go-gitea/go-sdk/gitea"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/context"
)

func AddCollaborator(ctx *context.APIContext, form api.AddCollaboratorOption) {
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
