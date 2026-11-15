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
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
		return
	}

	sortedColumnIDs := make(map[int64]int64)
	for _, column := range form.Columns {
		sortedColumnIDs[column.Sorting] = column.ColumnID
	}

	if err := project_model.MoveColumnsOnProject(ctx, project, sortedColumnIDs); err != nil {
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
		return
	}

	ctx.JSONOK()
}

func MoveIssues(ctx *context.Context) {
	project, column := findColumn(ctx)
	if ctx.Written() {
		return
	}

	form := &struct {
		IssueID     int64 `json:"issueID"`
		PrevIssueID int64 `json:"prevIssueID"`
	}{}
	if err := json.NewDecoder(ctx.Req.Body).Decode(form); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	issue, err := issues_model.GetIssueByID(ctx, form.IssueID)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if !project.CanBeAccessedByOwnerRepo(issue.Repo.OwnerID, issue.Repo) {
		ctx.ServerErrorAuto(errors.New("Some issue's repoID is not equal to project's repoID"))
		return
	}
	if err = project_service.MoveIssueAfter(ctx, ctx.Doer, column, issue, form.PrevIssueID); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	ctx.JSONOK()
}
