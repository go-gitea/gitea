// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/util"

	api "code.gitea.io/gitea/modules/structs"
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
	//   description: Milestone state, Recognised values are open, closed and all. Defaults to "open"
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/MilestoneList"
	milestones, err := models.GetMilestonesByRepoID(ctx.Repo.Repository.ID, api.StateType(ctx.Query("state")))
	if err != nil {
		ctx.Error(500, "GetMilestonesByRepoID", err)
		return
	}

	apiMilestones := make([]*api.Milestone, len(milestones))
	for i := range milestones {
		apiMilestones[i] = milestones[i].APIFormat()
	}
	ctx.JSON(200, &apiMilestones)
}

// GetMilestone get a milestone for a repository
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
	//   description: id of the milestone
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Milestone"
	milestone, err := models.GetMilestoneByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrMilestoneNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetMilestoneByRepoID", err)
		}
		return
	}
	ctx.JSON(200, milestone.APIFormat())
}

// CreateMilestone create a milestone for a repository
func CreateMilestone(ctx *context.APIContext, form api.CreateMilestoneOption) {
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
	if form.Deadline == nil {
		defaultDeadline, _ := time.ParseInLocation("2006-01-02", "9999-12-31", time.Local)
		form.Deadline = &defaultDeadline
	}

	milestone := &models.Milestone{
		RepoID:       ctx.Repo.Repository.ID,
		Name:         form.Title,
		Content:      form.Description,
		DeadlineUnix: util.TimeStamp(form.Deadline.Unix()),
	}

	if err := models.NewMilestone(milestone); err != nil {
		ctx.Error(500, "NewMilestone", err)
		return
	}
	ctx.JSON(201, milestone.APIFormat())
}

// EditMilestone modify a milestone for a repository
func EditMilestone(ctx *context.APIContext, form api.EditMilestoneOption) {
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
	//   description: id of the milestone
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditMilestoneOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Milestone"
	milestone, err := models.GetMilestoneByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrMilestoneNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetMilestoneByRepoID", err)
		}
		return
	}

	if len(form.Title) > 0 {
		milestone.Name = form.Title
	}
	if form.Description != nil {
		milestone.Content = *form.Description
	}
	if form.Deadline != nil && !form.Deadline.IsZero() {
		milestone.DeadlineUnix = util.TimeStamp(form.Deadline.Unix())
	}

	if err := models.UpdateMilestone(milestone); err != nil {
		ctx.ServerError("UpdateMilestone", err)
		return
	}
	ctx.JSON(200, milestone.APIFormat())
}

// DeleteMilestone delete a milestone for a repository
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
	//   description: id of the milestone to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	if err := models.DeleteMilestoneByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id")); err != nil {
		ctx.Error(500, "DeleteMilestoneByRepoID", err)
		return
	}
	ctx.Status(204)
}
