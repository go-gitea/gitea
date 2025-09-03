// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"fmt"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models/project"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
)

var (
	tmplRepoWorkflows = templates.TplName("repo/projects/workflows")
	tmplOrgWorkflows  = templates.TplName("org/projects/workflows")
)

func WorkflowsEvents(ctx *context.Context) {
	projectID := ctx.PathParamInt64("id")
	p, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.Type == project_model.TypeRepository && p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}
	if (p.Type == project_model.TypeOrganization || p.Type == project_model.TypeIndividual) && p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound(nil)
		return
	}

	workflows, err := project_model.FindWorkflowEvents(ctx, projectID)
	if err != nil {
		ctx.ServerError("GetWorkflows", err)
		return
	}
	type WorkflowEvent struct {
		EventID     string `json:"event_id"`
		DisplayName string `json:"display_name"`
	}
	outputWorkflows := make([]*WorkflowEvent, 0, len(workflows))
	events := project_model.GetWorkflowEvents()
	for _, event := range events {
		var workflow *WorkflowEvent
		for _, wf := range workflows {
			if wf.WorkflowEvent == event {
				workflow = &WorkflowEvent{
					EventID:     fmt.Sprintf("%d", wf.ID),
					DisplayName: string(ctx.Tr(wf.WorkflowEvent.LangKey())),
				}
				break
			}
		}
		if workflow == nil {
			workflow = &WorkflowEvent{
				EventID:     event.UUID(),
				DisplayName: string(ctx.Tr(event.LangKey())),
			}
		}
		outputWorkflows = append(outputWorkflows, workflow)
	}

	ctx.JSON(http.StatusOK, outputWorkflows)
}

func WorkflowsColumns(ctx *context.Context) {
	projectID := ctx.PathParamInt64("id")
	p, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.Type == project_model.TypeRepository && p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}
	if (p.Type == project_model.TypeOrganization || p.Type == project_model.TypeIndividual) && p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound(nil)
		return
	}

	columns, err := p.GetColumns(ctx)
	if err != nil {
		ctx.ServerError("GetProjectColumns", err)
		return
	}

	type Column struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}
	outputColumns := make([]*Column, 0, len(columns))
	for _, col := range columns {
		outputColumns = append(outputColumns, &Column{
			ID:    col.ID,
			Title: col.Title,
		})
	}

	ctx.JSON(http.StatusOK, outputColumns)
}

func Workflows(ctx *context.Context) {
	workflowIDStr := ctx.PathParam("workflow_id")
	if workflowIDStr == "events" {
		WorkflowsEvents(ctx)
		return
	}
	if workflowIDStr == "columns" {
		WorkflowsColumns(ctx)
		return
	}

	ctx.Data["WorkflowEvents"] = project_model.GetWorkflowEvents()

	projectID := ctx.PathParamInt64("id")
	p, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.Type == project_model.TypeRepository && p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}
	if (p.Type == project_model.TypeOrganization || p.Type == project_model.TypeIndividual) && p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["Title"] = ctx.Tr("projects.workflows")
	ctx.Data["PageIsWorkflows"] = true
	ctx.Data["PageIsProjects"] = true
	ctx.Data["PageIsProjectsWorkflows"] = true
	ctx.Data["Project"] = p

	workflows, err := project_model.FindWorkflowEvents(ctx, projectID)
	if err != nil {
		ctx.ServerError("GetWorkflows", err)
		return
	}
	for _, wf := range workflows {
		wf.Project = p
	}
	ctx.Data["Workflows"] = workflows

	ctx.Data["workflowIDStr"] = workflowIDStr
	var curWorkflow *project_model.Workflow
	if workflowIDStr == "" { // get first value workflow or the first workflow
		for _, wf := range workflows {
			if wf.ID > 0 {
				curWorkflow = wf
				break
			}
		}
	} else {
		workflowID, _ := strconv.ParseInt(workflowIDStr, 10, 64)
		if workflowID > 0 {
			for _, wf := range workflows {
				if wf.ID == workflowID {
					curWorkflow = wf
					break
				}
			}
		}
	}
	ctx.Data["CurWorkflow"] = curWorkflow
	ctx.Data["ProjectLink"] = project.ProjectLinkForRepo(ctx.Repo.Repository, projectID)

	if p.Type == project_model.TypeRepository {
		ctx.HTML(200, tmplRepoWorkflows)
	} else {
		ctx.HTML(200, tmplOrgWorkflows)
	}
}

type WorkflowsPostForm struct {
	EventID string            `form:"event_id" binding:"Required"`
	Filters map[string]string `form:"filters"`
	Actions map[string]any    `form:"actions"`
}

func WorkflowsPost(ctx *context.Context) {
	projectID := ctx.PathParamInt64("id")
	p, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.Type == project_model.TypeRepository && p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}
	if (p.Type == project_model.TypeOrganization || p.Type == project_model.TypeIndividual) && p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound(nil)
		return
	}

	form := web.GetForm(ctx).(*WorkflowsPostForm)
	eventID, _ := strconv.ParseInt(form.EventID, 10, 64)
	if eventID == 0 {
		// Create a new workflow
		wf := &project_model.Workflow{
			ProjectID:       projectID,
			WorkflowEvent:   project_model.WorkflowEvent(form.EventID),
			WorkflowFilters: []project_model.WorkflowFilter{},
			WorkflowActions: []project_model.WorkflowAction{},
		}
		if err := project_model.CreateWorkflow(ctx, wf); err != nil {
			ctx.ServerError("CreateWorkflow", err)
			return
		}
	} else {
		// Update an existing workflow
		wf, err := project_model.GetWorkflowByID(ctx, eventID)
		if err != nil {
			ctx.ServerError("GetWorkflowByID", err)
			return
		}
		wf.WorkflowFilters = []project_model.WorkflowFilter{}
		wf.WorkflowActions = []project_model.WorkflowAction{}
		if err := project_model.UpdateWorkflow(ctx, wf); err != nil {
			ctx.ServerError("UpdateWorkflow", err)
			return
		}
	}
}
