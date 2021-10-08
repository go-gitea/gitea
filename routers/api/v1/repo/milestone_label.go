// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
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
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabelList(m.Labels, ctx.Repo.Repository, ctx.Repo.Owner))
}

// AddMilestoneLabels add labels for a milestone
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
	m, labels, err := prepareMilestoneForReplaceOrAdd(ctx, *form)
	if err != nil {
		return
	}

	if err = m.AddLabels(ctx.User, labels); err != nil {
		ctx.Error(http.StatusInternalServerError, "AddLabels", err)
		return
	}

	labels, err = models.GetLabelsByMilestoneID(m.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByMilestoneID", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabelList(labels, ctx.Repo.Repository, ctx.Repo.Owner))
}

// DeleteMilestoneLabel delete a label for a milestone
func DeleteMilestoneLabel(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/milestones/{id}/labels/{id} milestone milestoneRemoveLabel
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
	// - name: id
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

	label, err := models.GetLabelByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrLabelNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetLabelByID", err)
		}
		return
	}

	if err := m.ReplaceLabels([]*models.Label{label}, ctx.User); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteIssueLabel", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ReplaceMilestoneLabels replace labels for a milestone
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
	m, labels, err := prepareMilestoneForReplaceOrAdd(ctx, *form)
	if err != nil {
		return
	}

	if err := m.ReplaceLabels(labels, ctx.User); err != nil {
		ctx.Error(http.StatusInternalServerError, "ReplaceLabels", err)
		return
	}

	labels, err = models.GetLabelsByMilestoneID(m.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByMilestoneID", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToLabelList(labels, ctx.Repo.Repository, ctx.Repo.Owner))
}

// ClearMilestoneLabels delete all the labels for a milestone
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

	if err := m.ClearLabels(ctx.User); err != nil {
		ctx.Error(http.StatusInternalServerError, "ClearLabels", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func prepareMilestoneForReplaceOrAdd(ctx *context.APIContext, form api.MilestoneLabelsOption) (milestone *models.Milestone, labels []*models.Label, err error) {
	milestone = getMilestoneByIDOrName(ctx)
	if milestone == nil {
		ctx.NotFound()
		return
	}

	labels, err = models.GetLabelsByIDs(form.Labels)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByIDs", err)
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(true) {
		ctx.Status(http.StatusForbidden)
		return
	}

	return
}
