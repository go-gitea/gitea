// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/label"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// getRepoLabelByIDOrName resolves a repo label from the path param, accepting either a numeric ID or a label name.
// Name lookup matches GetLabel; EditLabel/DeleteLabel previously only accepted numeric IDs (#34937).
func getRepoLabelByIDOrName(ctx *context.APIContext) (*issues_model.Label, error) {
	strID := ctx.PathParam("id")
	if intID, err := strconv.ParseInt(strID, 10, 64); err != nil {
		return issues_model.GetLabelInRepoByName(ctx, ctx.Repo.Repository.ID, strID)
	} else {
		return issues_model.GetLabelInRepoByID(ctx, ctx.Repo.Repository.ID, intID)
	}
}

// ListLabels list all the labels of a repository
func ListLabels(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/labels issue issueListLabels
	// ---
	// summary: Get all of a repository's labels
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
	//     "$ref": "#/responses/LabelList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	labels, err := issues_model.GetLabelsByRepoID(ctx, ctx.Repo.Repository.ID, ctx.FormString("sort"), utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	count, err := issues_model.CountLabelsByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, convert.ToLabelList(labels, ctx.Repo.Repository, nil))
}

// GetLabel get label by repository and label id or name
func GetLabel(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/labels/{id} issue issueGetLabel
	// ---
	// summary: Get a single label
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
	// - name: id
	//   in: path
	//   description: id or name of the label to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Label"
	//   "404":
	//     "$ref": "#/responses/notFound"

	l, err := getRepoLabelByIDOrName(ctx)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabel(l, ctx.Repo.Repository, nil))
}

// CreateLabel create a label for a repository
func CreateLabel(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/labels issue issueCreateLabel
	// ---
	// summary: Create a label
	// consumes:
	// - application/json
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateLabelOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Label"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateLabelOption)

	color, err := label.NormalizeColor(form.Color)
	if err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		return
	}
	form.Color = color
	l := &issues_model.Label{
		Name:        form.Name,
		Exclusive:   form.Exclusive,
		Color:       form.Color,
		RepoID:      ctx.Repo.Repository.ID,
		Description: form.Description,
	}
	l.SetArchived(form.IsArchived)
	if err := issues_model.NewLabel(ctx, l); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToLabel(l, ctx.Repo.Repository, nil))
}

// EditLabel modify a label for a repository
func EditLabel(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/labels/{id} issue issueEditLabel
	// ---
	// summary: Update a label
	// consumes:
	// - application/json
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
	// - name: id
	//   in: path
	//   description: id or name of the label to edit
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditLabelOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Label"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.EditLabelOption)
	l, err := getRepoLabelByIDOrName(ctx)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	if form.Name != nil {
		l.Name = *form.Name
	}
	if form.Exclusive != nil {
		l.Exclusive = *form.Exclusive
	}
	if form.Color != nil {
		color, err := label.NormalizeColor(*form.Color)
		if err != nil {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
			return
		}
		l.Color = color
	}
	if form.Description != nil {
		l.Description = *form.Description
	}
	l.SetArchived(form.IsArchived != nil && *form.IsArchived)
	if err := issues_model.UpdateLabel(ctx, l); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabel(l, ctx.Repo.Repository, nil))
}

// DeleteLabel delete a label for a repository
func DeleteLabel(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/labels/{id} issue issueDeleteLabel
	// ---
	// summary: Delete a label
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
	// - name: id
	//   in: path
	//   description: id or name of the label to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	l, err := getRepoLabelByIDOrName(ctx)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	if err := issues_model.DeleteLabel(ctx, ctx.Repo.Repository.ID, l.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
