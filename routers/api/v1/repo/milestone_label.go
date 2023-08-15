// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/convert"
)

// ListMilestoneLabels list all the labels of a milestones
func ListMilestoneLabels(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/milestones/{id}/labels milestones milestonesGetLabels
	// ---
	// summary: Get a milestone's labels
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
	//   description: milestone ID
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	m := getMilestoneByIDOrName(ctx)
	if err := m.LoadLabels(db.DefaultContext); err != nil {
		return
	}
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabelList(m.Labels, ctx.Repo.Repository, ctx.Repo.Owner))
}

// AddMilestoneLabels adds labels to a milestone
func AddMilestoneLabels(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/milestones/{id}/labels milestone milestoneAddLabel
	// ---
	// summary: Add a label to a milestone
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
	//   description: milestone ID
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MilestoneLabelsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	form := web.GetForm(ctx).(*api.MilestoneLabelsOption)
	m, selectLabels, err := prepareMilestoneForReplaceOrAdd(ctx, *form)
	if err != nil {
		return
	}

	if err = m.AddLabels(ctx.ContextUser, selectLabels); err != nil {
		ctx.Error(http.StatusInternalServerError, "AddLabels", err)
		return
	}

	selectLabels, err = issues_model.GetLabelsByMilestoneID(ctx, m.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByMilestoneID", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabelList(selectLabels, ctx.Repo.Repository, ctx.Repo.Owner))
}

// DeleteMilestoneLabel removes a label from a milestone
func DeleteMilestoneLabel(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/milestones/{id}/labels/{labelId} milestone milestoneRemoveLabel
	// ---
	// summary: Remove a label from a milestone
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
	//   description: milestone ID
	//   type: integer
	//   format: int64
	//   required: true
	// - name: labelId
	//   in: path
	//   description: id of the label to remove
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	m := getMilestoneByIDOrName(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(true) {
		ctx.Status(http.StatusForbidden)
		return
	}

	label, err := issues_model.GetLabelByID(ctx, ctx.ParamsInt64(":labelId"))
	if err != nil {
		if issues_model.IsErrLabelNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetLabelByID", err)
		}
		return
	}

	if err := m.ReplaceLabels([]*issues_model.Label{label}, ctx.ContextUser); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteIssueLabel", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ReplaceMilestoneLabels replaces labels on a milestone
func ReplaceMilestoneLabels(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/milestones/{id}/labels milestone milestoneReplaceLabels
	// ---
	// summary: Drop all previous milestone labels and replace them with new labels
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
	//   description: milestone ID
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MilestoneLabelsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	form := web.GetForm(ctx).(*api.MilestoneLabelsOption)
	m, selectLabels, err := prepareMilestoneForReplaceOrAdd(ctx, *form)
	if err != nil {
		return
	}

	if err := m.ReplaceLabels(selectLabels, ctx.ContextUser); err != nil {
		ctx.Error(http.StatusInternalServerError, "ReplaceLabels", err)
		return
	}

	selectLabels, err = issues_model.GetLabelsByMilestoneID(ctx, m.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByMilestoneID", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabelList(selectLabels, ctx.Repo.Repository, ctx.Repo.Owner))
}

// ClearMilestoneLabels removes all labels from a milestone
func ClearMilestoneLabels(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/milestones/{id}/labels milestone milestoneClearLabels
	// ---
	// summary: Remove all labels from an milestone
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
	//   description: milestone ID
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	m := getMilestoneByIDOrName(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(true) {
		ctx.Status(http.StatusForbidden)
		return
	}

	if err := m.ClearLabels(ctx.ContextUser); err != nil {
		ctx.Error(http.StatusInternalServerError, "ClearLabels", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func prepareMilestoneForReplaceOrAdd(ctx *context.APIContext, form api.MilestoneLabelsOption) (milestone *issues_model.Milestone, labels []*issues_model.Label, err error) {
	milestone = getMilestoneByIDOrName(ctx)
	if milestone == nil {
		ctx.NotFound()
		return
	}

	labels, err = issues_model.GetLabelsByIDs(form.Labels)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByIDs", err)
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(true) {
		ctx.Status(http.StatusForbidden)
		return
	}

	return milestone, labels, err
}
