// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// ListMilestones list milestones for a repository
func ListMilestones(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/milestones issue issueGetMilestonesList
	// ---
	// summary: Get all of a repository's opened milestones
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
	// - name: state
	//   in: query
	//   description: Milestone state, Recognized values are open, closed and all. Defaults to "open"
	//   type: string
	// - name: name
	//   in: query
	//   description: filter by milestone name
	//   type: string
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
	//     "$ref": "#/responses/MilestoneList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	state := api.StateType(ctx.FormString("state"))
	var isClosed optional.Option[bool]
	switch state {
	case api.StateClosed, api.StateOpen:
		isClosed = optional.Some(state == api.StateClosed)
	}

	milestones, total, err := db.FindAndCount[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
		IsClosed:    isClosed,
		Name:        ctx.FormString("name"),
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiMilestones := make([]*api.Milestone, len(milestones))
	for i := range milestones {
		apiMilestones[i] = convert.ToAPIMilestone(milestones[i])
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &apiMilestones)
}

// GetMilestone get a milestone for a repository by ID and if not available by name
func GetMilestone(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/milestones/{id} issue issueGetMilestone
	// ---
	// summary: Get a milestone
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
	//   description: the milestone to get, identified by ID and if not available by name
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Milestone"
	//   "404":
	//     "$ref": "#/responses/notFound"

	milestone := getMilestoneByIDOrName(ctx)
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIMilestone(milestone))
}

// CreateMilestone create a milestone for a repository
func CreateMilestone(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/milestones issue issueCreateMilestone
	// ---
	// summary: Create a milestone
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
	//     "$ref": "#/definitions/CreateMilestoneOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Milestone"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.CreateMilestoneOption)

	var deadlineUnix int64
	if form.Deadline != nil {
		deadlineUnix = form.Deadline.Unix()
	}

	milestone := &issues_model.Milestone{
		RepoID:       ctx.Repo.Repository.ID,
		Name:         form.Title,
		Content:      form.Description,
		DeadlineUnix: timeutil.TimeStamp(deadlineUnix),
	}

	if form.State == "closed" {
		milestone.IsClosed = true
		milestone.ClosedDateUnix = timeutil.TimeStampNow()
	}

	if err := issues_model.NewMilestone(ctx, milestone); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIMilestone(milestone))
}

// EditMilestone modify a milestone for a repository by ID and if not available by name
func EditMilestone(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/milestones/{id} issue issueEditMilestone
	// ---
	// summary: Update a milestone
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
	//   description: the milestone to edit, identified by ID and if not available by name
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditMilestoneOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Milestone"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.EditMilestoneOption)
	milestone := getMilestoneByIDOrName(ctx)
	if ctx.Written() {
		return
	}

	if len(form.Title) > 0 {
		milestone.Name = form.Title
	}
	if form.Description != nil {
		milestone.Content = *form.Description
	}
	milestone.DeadlineUnix, _ = common.ParseAPIDeadlineToEndOfDay(form.Deadline)

	oldIsClosed := milestone.IsClosed
	if form.State != nil {
		milestone.IsClosed = *form.State == string(api.StateClosed)
	}

	if err := issues_model.UpdateMilestone(ctx, milestone, oldIsClosed); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIMilestone(milestone))
}

// DeleteMilestone delete a milestone for a repository by ID and if not available by name
func DeleteMilestone(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/milestones/{id} issue issueDeleteMilestone
	// ---
	// summary: Delete a milestone
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
	//   description: the milestone to delete, identified by ID and if not available by name
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	m := getMilestoneByIDOrName(ctx)
	if ctx.Written() {
		return
	}

	if err := issues_model.DeleteMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, m.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// getMilestoneByIDOrName get milestone by ID and if not available by name
func getMilestoneByIDOrName(ctx *context.APIContext) *issues_model.Milestone {
	mile := ctx.PathParam("id")
	mileID, _ := strconv.ParseInt(mile, 0, 64)

	if mileID != 0 {
		milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, mileID)
		if err == nil {
			return milestone
		} else if !issues_model.IsErrMilestoneNotExist(err) {
			ctx.APIErrorInternal(err)
			return nil
		}
	}

	milestone, err := issues_model.GetMilestoneByRepoIDANDName(ctx, ctx.Repo.Repository.ID, mile)
	if err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.APIErrorNotFound()
			return nil
		}
		ctx.APIErrorInternal(err)
		return nil
	}

	return milestone
}
