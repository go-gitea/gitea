// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"strconv"
	"strings"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/label"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// getOrgLabelByIDOrName resolves an org label from the path param, accepting either a numeric ID or a label name.
// Name lookup matches GetLabel; EditLabel/DeleteLabel previously only accepted numeric IDs (#34937).
func getOrgLabelByIDOrName(ctx *context.APIContext) (*issues_model.Label, error) {
	strID := ctx.PathParam("id")
	if intID, err := strconv.ParseInt(strID, 10, 64); err != nil {
		return issues_model.GetLabelInOrgByName(ctx, ctx.Org.Organization.ID, strID)
	} else {
		return issues_model.GetLabelInOrgByID(ctx, ctx.Org.Organization.ID, intID)
	}
}

// writeOrgLabelNotFound maps org label not-found errors to 404 (same as GetLabel).
func writeOrgLabelErr(ctx *context.APIContext, err error) {
	if issues_model.IsErrOrgLabelNotExist(err) {
		ctx.APIErrorNotFound()
	} else {
		ctx.APIErrorInternal(err)
	}
}

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
	//   "404":
	//     "$ref": "#/responses/notFound"

	labels, err := issues_model.GetLabelsByOrgID(ctx, ctx.Org.Organization.ID, ctx.FormString("sort"), utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	count, err := issues_model.CountLabelsByOrgID(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.CreateLabelOption)
	form.Color = strings.Trim(form.Color, " ")
	color, err := label.NormalizeColor(form.Color)
	if err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		return
	}
	form.Color = color

	label := &issues_model.Label{
		Name:        form.Name,
		Exclusive:   form.Exclusive,
		Color:       form.Color,
		OrgID:       ctx.Org.Organization.ID,
		Description: form.Description,
	}
	if err := issues_model.NewLabel(ctx, label); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToLabel(label, nil, ctx.Org.Organization.AsUser()))
}

// GetLabel get label by organization and label id or name
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
	//   description: id or name of the label to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Label"
	//   "404":
	//     "$ref": "#/responses/notFound"

	label, err := getOrgLabelByIDOrName(ctx)
	if err != nil {
		writeOrgLabelErr(ctx, err)
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
	l, err := getOrgLabelByIDOrName(ctx)
	if err != nil {
		writeOrgLabelErr(ctx, err)
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

	ctx.JSON(http.StatusOK, convert.ToLabel(l, nil, ctx.Org.Organization.AsUser()))
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
	//   description: id or name of the label to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	l, err := getOrgLabelByIDOrName(ctx)
	if err != nil {
		writeOrgLabelErr(ctx, err)
		return
	}

	if err := issues_model.DeleteLabel(ctx, ctx.Org.Organization.ID, l.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
