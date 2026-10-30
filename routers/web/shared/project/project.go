// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"errors"
	"net/http"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/modules/json"
	"gitea.dev/modules/web"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	project_service "gitea.dev/services/projects"
)

// findProject loads the "id" path param, scoped to whichever owner the route assigned:
// anyone else's ID reads as not found. Write permission is enforced by the route.
func findProject(ctx *context.Context) *project_model.Project {
	var project *project_model.Project
	var err error
	if ctx.Repo != nil && ctx.Repo.Repository != nil {
		project, err = project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	} else {
		project, err = project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), ctx.ContextUser.ID)
	}
	if err != nil {
		ctx.ServerError("GetProject", err)
		return nil
	}
	return project
}

func findColumn(ctx *context.Context) (*project_model.Project, *project_model.Column) {
	project := findProject(ctx)
	if ctx.Written() {
		return nil, nil
	}
	column, err := project_model.GetColumnByIDAndProjectID(ctx, ctx.PathParamInt64("columnID"), project.ID)
	if err != nil {
		ctx.ServerError("GetColumnByIDAndProjectID", err)
		return nil, nil
	}
	return project, column
}

func MoveColumns(ctx *context.Context) {
	project := findProject(ctx)
	if ctx.Written() {
		return
	}

	type movedColumnsForm struct {
		Columns []struct {
			ColumnID int64 `json:"columnID"`
			Sorting  int64 `json:"sorting"`
		} `json:"columns"`
	}

	form := &movedColumnsForm{}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.ServerError("DecodeMovedColumnsForm", err)
		return
	}

	sortedColumnIDs := make(map[int64]int64)
	for _, column := range form.Columns {
		sortedColumnIDs[column.Sorting] = column.ColumnID
	}

	if err := project_model.MoveColumnsOnProject(ctx, project, sortedColumnIDs); err != nil {
		ctx.ServerError("MoveColumnsOnProject", err)
		return
	}

	ctx.JSONOK()
}

func AddColumnToProjectPost(ctx *context.Context) {
	form := web.GetForm[*forms.EditProjectColumnForm](ctx)
	project := findProject(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.NewColumn(ctx, &project_model.Column{
		ProjectID: project.ID,
		Title:     form.Title,
		Color:     form.Color,
		CreatorID: ctx.Doer.ID,
	}); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}

	ctx.JSONOK()
}

func EditProjectColumn(ctx *context.Context) {
	form := web.GetForm[*forms.EditProjectColumnForm](ctx)
	_, column := findColumn(ctx)
	if ctx.Written() {
		return
	}

	if form.Title != "" {
		column.Title = form.Title
	}
	column.Color = form.Color
	if form.Sorting != 0 {
		column.Sorting = form.Sorting
	}

	if err := project_model.UpdateColumn(ctx, column); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}

	ctx.JSONOK()
}

func DeleteProjectColumn(ctx *context.Context) {
	_, column := findColumn(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.DeleteColumnByID(ctx, column.ID); err != nil {
		ctx.ServerError("DeleteProjectColumnByID", err)
		return
	}

	ctx.JSONOK()
}

func SetDefaultProjectColumn(ctx *context.Context) {
	project, column := findColumn(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.SetDefaultColumn(ctx, project.ID, column.ID); err != nil {
		ctx.ServerError("SetDefaultColumn", err)
		return
	}

	ctx.JSONOK()
}

func MoveIssues(ctx *context.Context) {
	project, column := findColumn(ctx)
	if ctx.Written() {
		return
	}

	type movedIssuesForm struct {
		Issues []struct {
			IssueID int64 `json:"issueID"`
			Sorting int64 `json:"sorting"`
		} `json:"issues"`
	}

	form := &movedIssuesForm{}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.ServerError("DecodeMovedIssuesForm", err)
		return
	}

	issueIDs := make([]int64, 0, len(form.Issues))
	sortedIssueIDs := make(map[int64]int64)
	for _, issue := range form.Issues {
		issueIDs = append(issueIDs, issue.IssueID)
		sortedIssueIDs[issue.Sorting] = issue.IssueID
	}
	movedIssues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	if len(movedIssues) != len(form.Issues) {
		ctx.ServerError("some issues do not exist", errors.New("some issues do not exist"))
		return
	}

	if _, err = movedIssues.LoadRepositories(ctx); err != nil {
		ctx.ServerError("LoadRepositories", err)
		return
	}

	for _, issue := range movedIssues {
		if !project.CanBeAccessedByOwnerRepo(issue.Repo.OwnerID, issue.Repo) {
			ctx.ServerError("Some issue's repoID is not equal to project's repoID", errors.New("Some issue's repoID is not equal to project's repoID"))
			return
		}
	}

	if err = project_service.MoveIssuesOnProjectColumn(ctx, ctx.Doer, column, sortedIssueIDs); err != nil {
		ctx.ServerError("MoveIssuesOnProjectColumn", err)
		return
	}

	ctx.JSONOK()
}

type workflowItem struct {
	Event    project_model.WorkflowEvent
	Label    string
	Desc     string
	Workflow *project_model.Workflow
}

func workflowItems(workflows map[project_model.WorkflowEvent]*project_model.Workflow) []workflowItem {
	labels := map[project_model.WorkflowEvent][2]string{
		project_model.WorkflowEventItemAdded:         {"repo.projects.workflows.item_added", "repo.projects.workflows.item_added_desc"},
		project_model.WorkflowEventItemReopened:      {"repo.projects.workflows.item_reopened", "repo.projects.workflows.item_reopened_desc"},
		project_model.WorkflowEventItemClosed:        {"repo.projects.workflows.item_closed", "repo.projects.workflows.item_closed_desc"},
		project_model.WorkflowEventPullRequestMerged: {"repo.projects.workflows.pull_request_merged", "repo.projects.workflows.pull_request_merged_desc"},
		project_model.WorkflowEventChangesRequested:  {"repo.projects.workflows.changes_requested", "repo.projects.workflows.changes_requested_desc"},
		project_model.WorkflowEventReviewApproved:    {"repo.projects.workflows.review_approved", "repo.projects.workflows.review_approved_desc"},
		project_model.WorkflowEventCardMoved:         {"repo.projects.workflows.card_moved", "repo.projects.workflows.card_moved_desc"},
	}
	items := make([]workflowItem, 0, len(project_model.WorkflowEvents))
	for _, event := range project_model.WorkflowEvents {
		items = append(items, workflowItem{Event: event, Label: labels[event][0], Desc: labels[event][1], Workflow: workflows[event]})
	}
	return items
}

func RenderWorkflows(ctx *context.Context) {
	project := findProject(ctx)
	if ctx.Written() {
		return
	}
	columns, err := project_model.GetColumns(ctx, project.ID, db.ListOptionsAll)
	if err != nil {
		ctx.ServerError("GetColumns", err)
		return
	}
	workflows, err := project_model.GetWorkflowsByProjectID(ctx, project.ID)
	if err != nil {
		ctx.ServerError("GetWorkflowsByProjectID", err)
		return
	}

	ctx.Data["Title"] = ctx.Tr("repo.projects.workflows")
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["Project"] = project
	ctx.Data["Columns"] = columns
	ctx.Data["WorkflowItems"] = workflowItems(workflows)
	ctx.Data["CancelLink"] = project.Link(ctx)

	if ctx.Repo != nil && ctx.Repo.Repository != nil {
		ctx.HTML(http.StatusOK, "repo/projects/workflows")
		return
	}
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}
	ctx.HTML(http.StatusOK, "org/projects/workflows")
}

func WorkflowsPost(ctx *context.Context) {
	project := findProject(ctx)
	if ctx.Written() {
		return
	}

	for _, event := range project_model.WorkflowEvents {
		enabled := ctx.FormBool("enabled_" + string(event))
		columnID := ctx.FormInt64("column_" + string(event))
		if columnID == 0 {
			if enabled {
				ctx.Flash.Error(ctx.Tr("repo.projects.workflows.column_required"))
				ctx.Redirect(project.Link(ctx) + "/workflows")
				return
			}
			continue
		}
		if err := project_model.SaveWorkflow(ctx, project.ID, event, columnID, enabled); err != nil {
			ctx.ServerError("SaveWorkflow", err)
			return
		}
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.workflows.saved"))
	ctx.Redirect(project.Link(ctx))
}
