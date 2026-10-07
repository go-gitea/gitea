// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"net/http"
	"strconv"

	"gitea.dev/models/db"
	project_model "gitea.dev/models/project"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	project_service "gitea.dev/services/projects"
)

// RenderWorkflows renders the workflows page, the optional "workflow_key" path param is a workflow ID or an event
func RenderWorkflows(ctx *context.Context, canWrite bool, tpl templates.TplName) {
	project := findProject(ctx)
	if ctx.Written() {
		return
	}
	key := ctx.PathParam("workflow_key")
	if id, err := strconv.ParseInt(key, 10, 64); err == nil {
		if _, err := project_model.GetWorkflowByProjectAndID(ctx, project.ID, id); err != nil {
			ctx.NotFoundOrServerError("GetWorkflowByProjectAndID", db.IsErrNotExist, err)
			return
		}
	} else if key != "" && !project_model.WorkflowEvent(key).IsValid() {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["Title"] = ctx.Tr("projects.workflows")
	ctx.Data["Project"] = project
	ctx.Data["ProjectLink"] = project.Link(ctx)
	ctx.Data["WorkflowKey"] = key
	ctx.Data["CanWrite"] = canWrite
	ctx.HTML(http.StatusOK, tpl)
}

func WorkflowsData(ctx *context.Context) {
	project := findProject(ctx)
	if ctx.Written() {
		return
	}
	workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
	if err != nil {
		ctx.ServerError("FindWorkflowsByProjectID", err)
		return
	}
	columns, err := project_model.GetColumns(ctx, project.ID, db.ListOptionsAll)
	if err != nil {
		ctx.ServerError("GetColumns", err)
		return
	}
	labels, err := project_service.GetProjectLabels(ctx, project)
	if err != nil {
		ctx.ServerError("GetProjectLabels", err)
		return
	}

	type workflowEvent struct {
		Event       project_model.WorkflowEvent        `json:"event"`
		DisplayName string                             `json:"display_name"`
		Filters     []project_model.WorkflowFilterType `json:"filters"`
		Actions     []project_model.WorkflowActionType `json:"actions"`
	}
	events := make([]workflowEvent, 0, len(project_model.WorkflowEvents()))
	for _, event := range project_model.WorkflowEvents() {
		capabilities := event.Capabilities()
		events = append(events, workflowEvent{event, ctx.Locale.TrString(event.LangKey()), capabilities.Filters, capabilities.Actions})
	}

	type workflowColumn struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}
	workflowColumns := make([]workflowColumn, 0, len(columns))
	for _, column := range columns {
		workflowColumns = append(workflowColumns, workflowColumn{column.ID, column.Title})
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"events":    events,
		"workflows": convert.ToProjectWorkflowList(workflows),
		"columns":   workflowColumns,
		"labels":    convert.ToLabelList(labels, ctx.Repo.Repository, ctx.ContextUser),
	})
}

func CreateWorkflow(ctx *context.Context) {
	project := findProject(ctx)
	if ctx.Written() {
		return
	}
	var form api.CreateProjectWorkflowOption
	if err := json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.JSONError("invalid request body")
		return
	}
	workflow, err := project_service.CreateWorkflow(ctx, ctx.Doer, project, &form)
	if err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProjectWorkflow(workflow))
}

func UpdateWorkflow(ctx *context.Context) {
	project, workflow := findWorkflow(ctx)
	if ctx.Written() {
		return
	}
	var form api.EditProjectWorkflowOption
	if err := json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.JSONError("invalid request body")
		return
	}
	if err := project_service.UpdateWorkflow(ctx, ctx.Doer, project, workflow, &form); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProjectWorkflow(workflow))
}

func DeleteWorkflow(ctx *context.Context) {
	project, workflow := findWorkflow(ctx)
	if ctx.Written() {
		return
	}
	if err := project_model.DeleteWorkflow(ctx, project.ID, workflow.ID); err != nil {
		ctx.ServerError("DeleteWorkflow", err)
		return
	}
	ctx.JSONOK()
}

func findWorkflow(ctx *context.Context) (*project_model.Project, *project_model.Workflow) {
	project := findProject(ctx)
	if ctx.Written() {
		return nil, nil
	}
	workflow, err := project_model.GetWorkflowByProjectAndID(ctx, project.ID, ctx.PathParamInt64("workflow_id"))
	if err != nil {
		ctx.JSONErrorAuto(err)
		return nil, nil
	}
	return project, workflow
}
