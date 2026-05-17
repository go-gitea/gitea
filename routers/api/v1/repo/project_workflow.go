// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	api_context "code.gitea.io/gitea/services/context"
	project_service "code.gitea.io/gitea/services/projects"
)

func getAPIProjectWorkflowProject(ctx *api_context.APIContext) (*project_model.Project, bool) {
	projectID := ctx.PathParamInt64("project_id")
	project, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, false
	}
	if project.Type != project_model.TypeRepository || project.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return nil, false
	}
	return project, true
}

func toAPIProjectWorkflowCapabilities(capabilities project_model.WorkflowEventCapabilities) api.ProjectWorkflowCapabilities {
	availableFilters := make([]string, 0, len(capabilities.AvailableFilters))
	for _, filter := range capabilities.AvailableFilters {
		availableFilters = append(availableFilters, string(filter))
	}
	availableActions := make([]string, 0, len(capabilities.AvailableActions))
	for _, action := range capabilities.AvailableActions {
		availableActions = append(availableActions, string(action))
	}
	return api.ProjectWorkflowCapabilities{
		AvailableFilters: availableFilters,
		AvailableActions: availableActions,
	}
}

func toAPIProjectWorkflowFilters(items []project_model.WorkflowFilter) []api.ProjectWorkflowRule {
	result := make([]api.ProjectWorkflowRule, 0, len(items))
	for _, item := range items {
		result = append(result, api.ProjectWorkflowRule{Type: string(item.Type), Value: item.Value})
	}
	return result
}

func toAPIProjectWorkflowActions(items []project_model.WorkflowAction) []api.ProjectWorkflowRule {
	result := make([]api.ProjectWorkflowRule, 0, len(items))
	for _, item := range items {
		result = append(result, api.ProjectWorkflowRule{Type: string(item.Type), Value: item.Value})
	}
	return result
}

func toAPIProjectWorkflow(ctx *api_context.APIContext, workflow *project_model.Workflow, capabilities project_model.WorkflowEventCapabilities) *api.ProjectWorkflow {
	return &api.ProjectWorkflow{
		ID:            workflow.ID,
		EventID:       string(workflow.WorkflowEvent),
		DisplayName:   string(ctx.Tr(workflow.WorkflowEvent.LangKey())),
		WorkflowEvent: string(workflow.WorkflowEvent),
		Capabilities:  toAPIProjectWorkflowCapabilities(capabilities),
		Filters:       toAPIProjectWorkflowFilters(workflow.WorkflowFilters),
		Actions:       toAPIProjectWorkflowActions(workflow.WorkflowActions),
		Summary:       project_service.GetWorkflowSummary(ctx, workflow),
		Enabled:       workflow.Enabled,
		IsConfigured:  true,
	}
}

func listAPIProjectWorkflows(ctx *api_context.APIContext, project *project_model.Project) ([]*api.ProjectWorkflow, error) {
	workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
	if err != nil {
		return nil, err
	}

	events := project_model.GetWorkflowEvents()
	capabilities := project_model.GetWorkflowEventCapabilities()
	workflowMap := make(map[project_model.WorkflowEvent][]*project_model.Workflow)
	for _, workflow := range workflows {
		workflowMap[workflow.WorkflowEvent] = append(workflowMap[workflow.WorkflowEvent], workflow)
	}

	result := make([]*api.ProjectWorkflow, 0, len(events))
	for _, event := range events {
		existing := workflowMap[event]
		if len(existing) > 0 {
			for _, workflow := range existing {
				result = append(result, toAPIProjectWorkflow(ctx, workflow, capabilities[event]))
			}
			continue
		}

		result = append(result, &api.ProjectWorkflow{
			ID:            0,
			EventID:       string(event),
			DisplayName:   string(ctx.Tr(event.LangKey())),
			WorkflowEvent: string(event),
			Capabilities:  toAPIProjectWorkflowCapabilities(capabilities[event]),
			Enabled:       true,
			IsConfigured:  false,
		})
	}

	return result, nil
}

func convertAPIProjectWorkflowFilters(ctx *api_context.APIContext, project *project_model.Project, options api.ProjectWorkflowFilterOptions) []project_model.WorkflowFilter {
	filters := make([]project_model.WorkflowFilter, 0)
	if options.IssueType != "" {
		filters = append(filters, project_model.WorkflowFilter{Type: project_model.WorkflowFilterTypeIssueType, Value: options.IssueType})
	}

	for _, item := range []struct {
		typeName project_model.WorkflowFilterType
		value    string
	}{
		{typeName: project_model.WorkflowFilterTypeSourceColumn, value: options.SourceColumn},
		{typeName: project_model.WorkflowFilterTypeTargetColumn, value: options.TargetColumn},
	} {
		if item.value == "" {
			continue
		}
		columnID, _ := strconv.ParseInt(item.value, 10, 64)
		if columnID <= 0 {
			continue
		}
		column, _ := project_model.GetColumnByIDAndProjectID(ctx, columnID, project.ID)
		if column == nil {
			continue
		}
		filters = append(filters, project_model.WorkflowFilter{Type: item.typeName, Value: strconv.FormatInt(columnID, 10)})
	}

	for _, label := range options.Labels {
		if label == "" {
			continue
		}
		labelID, _ := strconv.ParseInt(label, 10, 64)
		if project_service.CanProjectAddLabel(ctx, project, labelID) {
			filters = append(filters, project_model.WorkflowFilter{Type: project_model.WorkflowFilterTypeLabels, Value: label})
		}
	}

	return filters
}

func convertAPIProjectWorkflowActions(ctx *api_context.APIContext, project *project_model.Project, options api.ProjectWorkflowActionOptions) []project_model.WorkflowAction {
	actions := make([]project_model.WorkflowAction, 0)
	if options.Column != "" {
		columnID, _ := strconv.ParseInt(options.Column, 10, 64)
		if columnID > 0 {
			column, _ := project_model.GetColumnByIDAndProjectID(ctx, columnID, project.ID)
			if column != nil {
				actions = append(actions, project_model.WorkflowAction{Type: project_model.WorkflowActionTypeColumn, Value: strconv.FormatInt(columnID, 10)})
			}
		}
	}

	for _, entry := range []struct {
		typeName project_model.WorkflowActionType
		labels   []string
	}{
		{typeName: project_model.WorkflowActionTypeAddLabels, labels: options.AddLabels},
		{typeName: project_model.WorkflowActionTypeRemoveLabels, labels: options.RemoveLabels},
	} {
		for _, label := range entry.labels {
			if label == "" {
				continue
			}
			labelID, _ := strconv.ParseInt(label, 10, 64)
			if project_service.CanProjectAddLabel(ctx, project, labelID) {
				actions = append(actions, project_model.WorkflowAction{Type: entry.typeName, Value: label})
			}
		}
	}

	issueState := strings.ToLower(options.IssueState)
	if issueState == "close" || issueState == "reopen" {
		actions = append(actions, project_model.WorkflowAction{Type: project_model.WorkflowActionTypeIssueState, Value: issueState})
	}

	return actions
}

// ListProjectWorkflows lists the workflows for a repository project.
func ListProjectWorkflows(ctx *api_context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{project_id}/workflows repository ListProjectWorkflows
	// ---
	// summary: List project workflows
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
	// - name: project_id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflowList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	if project, ok := getAPIProjectWorkflowProject(ctx); ok {
		workflows, err := listAPIProjectWorkflows(ctx, project)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		ctx.JSON(http.StatusOK, workflows)
	}
}

// GetProjectWorkflow gets a configured project workflow.
func GetProjectWorkflow(ctx *api_context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{project_id}/workflows/{workflow_id} repository GetProjectWorkflow
	// ---
	// summary: Get a project workflow
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
	// - name: project_id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	project, ok := getAPIProjectWorkflowProject(ctx)
	if !ok {
		return
	}
	workflow, err := project_model.GetWorkflowByProjectAndID(ctx, project.ID, ctx.PathParamInt64("workflow_id"))
	if err != nil {
		if db.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	capabilities := project_model.GetWorkflowEventCapabilities()
	ctx.JSON(http.StatusOK, toAPIProjectWorkflow(ctx, workflow, capabilities[workflow.WorkflowEvent]))
}

// GetProjectWorkflowOptions gets the available project workflow options.
func GetProjectWorkflowOptions(ctx *api_context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{project_id}/workflows/options repository GetProjectWorkflowOptions
	// ---
	// summary: Get project workflow options
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
	// - name: project_id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflowOptions"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	project, ok := getAPIProjectWorkflowProject(ctx)
	if !ok {
		return
	}
	columns, err := project.GetColumns(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	labels, err := project_service.GetProjectLabels(ctx, project)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	options := &api.ProjectWorkflowOptions{
		Columns: make([]*api.ProjectWorkflowColumnOption, 0, len(columns)),
		Labels:  make([]*api.ProjectWorkflowLabelOption, 0, len(labels)),
	}
	for _, column := range columns {
		options.Columns = append(options.Columns, &api.ProjectWorkflowColumnOption{ID: column.ID, Title: column.Title, Color: column.Color})
	}
	for _, label := range labels {
		options.Labels = append(options.Labels, &api.ProjectWorkflowLabelOption{
			ID:             label.ID,
			Name:           label.Name,
			Color:          label.Color,
			Description:    label.Description,
			Exclusive:      label.Exclusive,
			ExclusiveOrder: label.ExclusiveOrder,
		})
	}
	ctx.JSON(http.StatusOK, options)
}

// CreateProjectWorkflow creates a project workflow.
func CreateProjectWorkflow(ctx *api_context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{project_id}/workflows repository CreateProjectWorkflow
	// ---
	// summary: Create a project workflow
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
	// - name: project_id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     type: object
	//     required:
	//       - event_id
	//     properties:
	//       event_id:
	//         type: string
	//       filters:
	//         type: object
	//         properties:
	//           issue_type:
	//             type: string
	//           source_column:
	//             type: string
	//           target_column:
	//             type: string
	//           labels:
	//             type: array
	//             items:
	//               type: string
	//       actions:
	//         type: object
	//         properties:
	//           column:
	//             type: string
	//           add_labels:
	//             type: array
	//             items:
	//               type: string
	//           remove_labels:
	//             type: array
	//             items:
	//               type: string
	//           issue_state:
	//             type: string
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	project, ok := getAPIProjectWorkflowProject(ctx)
	if !ok {
		return
	}
	form := web.GetForm(ctx).(*api.CreateProjectWorkflowOption)
	if !project_model.IsValidWorkflowEvent(form.EventID) {
		ctx.APIError(http.StatusUnprocessableEntity, "invalid event_id: "+form.EventID)
		return
	}
	workflow := &project_model.Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   project_model.WorkflowEvent(form.EventID),
		WorkflowFilters: convertAPIProjectWorkflowFilters(ctx, project, form.Filters),
		WorkflowActions: convertAPIProjectWorkflowActions(ctx, project, form.Actions),
		Enabled:         true,
	}
	if len(workflow.WorkflowActions) == 0 {
		ctx.APIError(http.StatusUnprocessableEntity, errors.New("at least one action is required"))
		return
	}
	if err := project_model.CreateWorkflow(ctx, workflow); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	capabilities := project_model.GetWorkflowEventCapabilities()
	ctx.JSON(http.StatusCreated, toAPIProjectWorkflow(ctx, workflow, capabilities[workflow.WorkflowEvent]))
}

// UpdateProjectWorkflow updates a project workflow.
func UpdateProjectWorkflow(ctx *api_context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/{project_id}/workflows/{workflow_id} repository UpdateProjectWorkflow
	// ---
	// summary: Update a project workflow
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
	// - name: project_id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     type: object
	//     properties:
	//       filters:
	//         type: object
	//         properties:
	//           issue_type:
	//             type: string
	//           source_column:
	//             type: string
	//           target_column:
	//             type: string
	//           labels:
	//             type: array
	//             items:
	//               type: string
	//       actions:
	//         type: object
	//         properties:
	//           column:
	//             type: string
	//           add_labels:
	//             type: array
	//             items:
	//               type: string
	//           remove_labels:
	//             type: array
	//             items:
	//               type: string
	//           issue_state:
	//             type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	project, ok := getAPIProjectWorkflowProject(ctx)
	if !ok {
		return
	}
	workflow, err := project_model.GetWorkflowByProjectAndID(ctx, project.ID, ctx.PathParamInt64("workflow_id"))
	if err != nil {
		if db.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	form := web.GetForm(ctx).(*api.EditProjectWorkflowOption)
	workflow.WorkflowFilters = convertAPIProjectWorkflowFilters(ctx, project, form.Filters)
	workflow.WorkflowActions = convertAPIProjectWorkflowActions(ctx, project, form.Actions)
	if len(workflow.WorkflowActions) == 0 {
		ctx.APIError(http.StatusUnprocessableEntity, errors.New("at least one action is required"))
		return
	}
	if err := project_model.UpdateWorkflow(ctx, workflow); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	capabilities := project_model.GetWorkflowEventCapabilities()
	ctx.JSON(http.StatusOK, toAPIProjectWorkflow(ctx, workflow, capabilities[workflow.WorkflowEvent]))
}

func changeProjectWorkflowEnabled(ctx *api_context.APIContext, enabled bool) {
	project, ok := getAPIProjectWorkflowProject(ctx)
	if !ok {
		return
	}
	workflowID := ctx.PathParamInt64("workflow_id")
	_, err := project_model.GetWorkflowByProjectAndID(ctx, project.ID, workflowID)
	if err != nil {
		if db.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	if enabled {
		err = project_model.EnableWorkflow(ctx, workflowID)
	} else {
		err = project_model.DisableWorkflow(ctx, workflowID)
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// EnableProjectWorkflow enables a project workflow.
func EnableProjectWorkflow(ctx *api_context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/projects/{project_id}/workflows/{workflow_id}/enable repository EnableProjectWorkflow
	// ---
	// summary: Enable a project workflow
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
	// - name: project_id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	changeProjectWorkflowEnabled(ctx, true)
}

// DisableProjectWorkflow disables a project workflow.
func DisableProjectWorkflow(ctx *api_context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/projects/{project_id}/workflows/{workflow_id}/disable repository DisableProjectWorkflow
	// ---
	// summary: Disable a project workflow
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
	// - name: project_id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	changeProjectWorkflowEnabled(ctx, false)
}

// DeleteProjectWorkflow deletes a project workflow.
func DeleteProjectWorkflow(ctx *api_context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{project_id}/workflows/{workflow_id} repository DeleteProjectWorkflow
	// ---
	// summary: Delete a project workflow
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
	// - name: project_id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	project, ok := getAPIProjectWorkflowProject(ctx)
	if !ok {
		return
	}
	workflow, err := project_model.GetWorkflowByProjectAndID(ctx, project.ID, ctx.PathParamInt64("workflow_id"))
	if err != nil {
		if db.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	if err := project_model.DeleteWorkflow(ctx, workflow.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
