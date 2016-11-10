// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "code.gitea.io/go-sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

func ListLabels(ctx *context.APIContext) {
	labels, err := models.GetLabelsByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Error(500, "GetLabelsByRepoID", err)
		return
	}

	apiLabels := make([]*api.Label, len(labels))
	for i := range labels {
		apiLabels[i] = labels[i].APIFormat()
	}
	ctx.JSON(200, &apiLabels)
}

func GetLabel(ctx *context.APIContext) {
	label, err := models.GetLabelInRepoByID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrLabelNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetLabelByRepoID", err)
		}
		return
	}

	ctx.JSON(200, label.APIFormat())
}

func CreateLabel(ctx *context.APIContext, form api.CreateLabelOption) {
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	label := &models.Label{
		Name:   form.Name,
		Color:  form.Color,
		RepoID: ctx.Repo.Repository.ID,
	}
	if err := models.NewLabels(label); err != nil {
		ctx.Error(500, "NewLabel", err)
		return
	}
	ctx.JSON(201, label.APIFormat())
}

func EditLabel(ctx *context.APIContext, form api.EditLabelOption) {
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	label, err := models.GetLabelInRepoByID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrLabelNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetLabelByRepoID", err)
		}
		return
	}

	if form.Name != nil {
		label.Name = *form.Name
	}
	if form.Color != nil {
		label.Color = *form.Color
	}
	if err := models.UpdateLabel(label); err != nil {
		ctx.Handle(500, "UpdateLabel", err)
		return
	}
	ctx.JSON(200, label.APIFormat())
}

func DeleteLabel(ctx *context.APIContext) {
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	if err := models.DeleteLabel(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id")); err != nil {
		ctx.Error(500, "DeleteLabel", err)
		return
	}

	ctx.Status(204)
}
