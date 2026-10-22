// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"html/template"
	"net/http"
	"slices"
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

func RenderWorkflows(ctx *context.Context, canWrite bool, tpl templates.TplName) {
	project := findProject(ctx)
	if ctx.Written() {
		return
	}
	if canWrite {
		var err error
		if canWrite, err = project_service.CanManageWorkflows(ctx, ctx.Doer, project); err != nil {
			ctx.ServerError("CanManageWorkflows", err)
			return
		}
	}
	workflows, err := db.Find[project_model.Workflow](ctx, project_model.FindWorkflowsOptions{ListOptions: db.ListOptionsAll, ProjectIDs: []int64{project.ID}})
	if err != nil {
		ctx.ServerError("FindWorkflows", err)
		return
	}
	var workflowID int64
	if param := ctx.PathParam("workflow_id"); param != "" {
		id, err := strconv.ParseInt(param, 10, 64)
		if err != nil || !slices.ContainsFunc(workflows, func(workflow *project_model.Workflow) bool { return workflow.ID == id }) {
			ctx.NotFound(nil)
			return
		}
		workflowID = id
	}
	event := ctx.FormString("event")
	if event != "" && !project_model.WorkflowEvent(event).IsValid() {
		ctx.NotFound(nil)
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
		Filters     []project_model.WorkflowFilterType `json:"supported_filters"`
		Actions     []project_model.WorkflowActionType `json:"supported_actions"`
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

	type workflowLabel struct {
		ID   int64         `json:"id"`
		Name string        `json:"name"`
		HTML template.HTML `json:"html"`
	}
	renderUtils := templates.NewRenderUtils(ctx)
	workflowLabels := make([]workflowLabel, 0, len(labels))
	for _, label := range labels {
		workflowLabels = append(workflowLabels, workflowLabel{label.ID, label.Name, renderUtils.RenderLabel(label)})
	}

	ctx.PageData["projectWorkflows"] = map[string]any{
		"events":    events,
		"workflows": convert.ToProjectWorkflowList(workflows),
		"columns":   workflowColumns,
		"labels":    workflowLabels,
	}
	ctx.Data["Title"] = ctx.Tr("projects.workflows")
	ctx.Data["Project"] = project
	ctx.Data["ProjectLink"] = project.Link(ctx)
	ctx.Data["WorkflowID"] = workflowID
	ctx.Data["WorkflowEvent"] = event
	ctx.Data["CanWrite"] = canWrite
	ctx.HTML(http.StatusOK, tpl)
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
	if err := project_service.DeleteWorkflow(ctx, ctx.Doer, project, workflow); err != nil {
		ctx.JSONErrorAuto(err)
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
