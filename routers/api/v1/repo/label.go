// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
)

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
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"

	labels, err := models.GetLabelsByRepoID(ctx.Repo.Repository.ID, ctx.Query("sort"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByRepoID", err)
		return
	}

	apiLabels := make([]*api.Label, len(labels))
	for i := range labels {
		apiLabels[i] = labels[i].APIFormat()
	}
	ctx.JSON(http.StatusOK, &apiLabels)
}

// GetLabel get label by repository and label id
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
	//   description: id of the label to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Label"

	var (
		label *models.Label
		err   error
	)
	strID := ctx.Params(":id")
	if intID, err2 := strconv.ParseInt(strID, 10, 64); err2 != nil {
		label, err = models.GetLabelInRepoByName(ctx.Repo.Repository.ID, strID)
	} else {
		label, err = models.GetLabelInRepoByID(ctx.Repo.Repository.ID, intID)
	}
	if err != nil {
		if models.IsErrLabelNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetLabelByRepoID", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, label.APIFormat())
}

// CreateLabel create a label for a repository
func CreateLabel(ctx *context.APIContext, form api.CreateLabelOption) {
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

	label := &models.Label{
		Name:        form.Name,
		Color:       form.Color,
		RepoID:      ctx.Repo.Repository.ID,
		Description: form.Description,
	}
	if err := models.NewLabel(label); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewLabel", err)
		return
	}
	ctx.JSON(http.StatusCreated, label.APIFormat())
}

// EditLabel modify a label for a repository
func EditLabel(ctx *context.APIContext, form api.EditLabelOption) {
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
	//   description: id of the label to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditLabelOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Label"

	label, err := models.GetLabelInRepoByID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrLabelNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetLabelByRepoID", err)
		}
		return
	}

	if form.Name != nil {
		label.Name = *form.Name
	}
	if form.Color != nil {
		label.Color = *form.Color
	}
	if form.Description != nil {
		label.Description = *form.Description
	}
	if err := models.UpdateLabel(label); err != nil {
		ctx.ServerError("UpdateLabel", err)
		return
	}
	ctx.JSON(http.StatusOK, label.APIFormat())
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
	//   description: id of the label to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	if err := models.DeleteLabel(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id")); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteLabel", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
