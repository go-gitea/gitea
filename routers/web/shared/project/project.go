// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"errors"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/modules/json"
	"gitea.dev/modules/web"
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
		ctx.NotFoundOrServerError("GetProject", project_model.IsErrProjectNotExist, err)
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
		ctx.NotFoundOrServerError("GetColumnByIDAndProjectID", project_model.IsErrProjectColumnNotExist, err)
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
		ctx.ServerError("NewProjectColumn", err)
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
		ctx.ServerError("UpdateProjectColumn", err)
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
		ctx.NotFoundOrServerError("GetIssueByID", issues_model.IsErrIssueNotExist, err)
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
