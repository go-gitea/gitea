// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ListLabels list all the labels of an organization
func ListLabels(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/labels organization orgListLabels
	// ---
	// summary: List an organization's labels
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
	//     "$ref": "#/responses/LabelList"

	labels, err := issues_model.GetLabelsByOrgID(ctx, ctx.Org.Organization.ID, ctx.FormString("sort"), utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByOrgID", err)
		return
	}

	count, err := issues_model.CountLabelsByOrgID(ctx.Org.Organization.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, convert.ToLabelList(labels, nil, ctx.Org.Organization.AsUser()))
}

// CreateLabel create a label for a repository
func CreateLabel(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/labels organization orgCreateLabel
	// ---
	// summary: Create a label for an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateLabelOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Label"
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.CreateLabelOption)
	form.Color = strings.Trim(form.Color, " ")
	if len(form.Color) == 6 {
		form.Color = "#" + form.Color
	}
	if !issues_model.LabelColorPattern.MatchString(form.Color) {
		ctx.Error(http.StatusUnprocessableEntity, "ColorPattern", fmt.Errorf("bad color code: %s", form.Color))
		return
	}

	label := &issues_model.Label{
		Name:        form.Name,
		Color:       form.Color,
		OrgID:       ctx.Org.Organization.ID,
		Description: form.Description,
	}
	if err := issues_model.NewLabel(ctx, label); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewLabel", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToLabel(label, nil, ctx.Org.Organization.AsUser()))
}

// GetLabel get label by organization and label id
func GetLabel(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/labels/{id} organization orgGetLabel
	// ---
	// summary: Get a single label
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
		label *issues_model.Label
		err   error
	)
	strID := ctx.Params(":id")
	if intID, err2 := strconv.ParseInt(strID, 10, 64); err2 != nil {
		label, err = issues_model.GetLabelInOrgByName(ctx, ctx.Org.Organization.ID, strID)
	} else {
		label, err = issues_model.GetLabelInOrgByID(ctx, ctx.Org.Organization.ID, intID)
	}
	if err != nil {
		if issues_model.IsErrOrgLabelNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetLabelByOrgID", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabel(label, nil, ctx.Org.Organization.AsUser()))
}

// EditLabel modify a label for an Organization
func EditLabel(ctx *context.APIContext) {
	// swagger:operation PATCH /orgs/{org}/labels/{id} organization orgEditLabel
	// ---
	// summary: Update a label
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.EditLabelOption)
	label, err := issues_model.GetLabelInOrgByID(ctx, ctx.Org.Organization.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if issues_model.IsErrOrgLabelNotExist(err) {
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
		label.Color = strings.Trim(*form.Color, " ")
		if len(label.Color) == 6 {
			label.Color = "#" + label.Color
		}
		if !issues_model.LabelColorPattern.MatchString(label.Color) {
			ctx.Error(http.StatusUnprocessableEntity, "ColorPattern", fmt.Errorf("bad color code: %s", label.Color))
			return
		}
	}
	if form.Description != nil {
		label.Description = *form.Description
	}
	if err := issues_model.UpdateLabel(label); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateLabel", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabel(label, nil, ctx.Org.Organization.AsUser()))
}

// DeleteLabel delete a label for an organization
func DeleteLabel(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/labels/{id} organization orgDeleteLabel
	// ---
	// summary: Delete a label
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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

	if err := issues_model.DeleteLabel(ctx.Org.Organization.ID, ctx.ParamsInt64(":id")); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteLabel", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
