// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	stdCtx "context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"gitea.dev/models/db"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unit"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	project_service "gitea.dev/services/projects"
)

var (
	tmplRepoWorkflows = templates.TplName("repo/projects/workflows")
	tmplOrgWorkflows  = templates.TplName("org/projects/workflows")
)

// convertFormToFilters converts form filters to WorkflowFilter objects
func convertFormToFilters(ctx stdCtx.Context, project *project_model.Project, event project_model.WorkflowEvent, formFilters map[string]any) []project_model.WorkflowFilter {
	filters := make([]project_model.WorkflowFilter, 0)

	caps := project_model.GetWorkflowEventCapabilities()[event]
	allowed := make(map[project_model.WorkflowFilterType]bool, len(caps.AvailableFilters))
	for _, ft := range caps.AvailableFilters {
		allowed[ft] = true
	}

	for key, value := range formFilters {
		filterType := project_model.WorkflowFilterType(key)
		if !allowed[filterType] {
			continue // not supported for this event
		}
		switch filterType {
		case project_model.WorkflowFilterTypeLabels:
			// Handle labels array
			if labelInterfaces, ok := value.([]any); ok && len(labelInterfaces) > 0 {
				for _, labelInterface := range labelInterfaces {
					if label, ok := labelInterface.(string); ok && label != "" {
						labelID, _ := strconv.ParseInt(label, 10, 64)
						if project_service.CanProjectAddLabel(ctx, project, labelID) {
							filters = append(filters, project_model.WorkflowFilter{
								Type:  filterType,
								Value: label,
							})
						}
					}
				}
			}
		case project_model.WorkflowFilterTypeSourceColumn, project_model.WorkflowFilterTypeTargetColumn:
			if strValue, ok := value.(string); ok && strValue != "" {
				strValueInt, _ := strconv.ParseInt(strValue, 10, 64)
				if strValueInt > 0 {
					col, _ := project_model.GetColumnByIDAndProjectID(ctx, strValueInt, project.ID)
					if col == nil {
						continue
					}
					filters = append(filters, project_model.WorkflowFilter{
						Type:  filterType,
						Value: strconv.FormatInt(strValueInt, 10),
					})
				}
			}
		default:
			// Handle string values (issue_type, column)
			if strValue, ok := value.(string); ok && strValue != "" {
				filters = append(filters, project_model.WorkflowFilter{
					Type:  filterType,
					Value: strValue,
				})
			}
		}
	}

	return filters
}

// convertFormToActions converts form actions to WorkflowAction objects
func convertFormToActions(ctx stdCtx.Context, project *project_model.Project, event project_model.WorkflowEvent, formActions map[string]any) []project_model.WorkflowAction {
	actions := make([]project_model.WorkflowAction, 0)

	caps := project_model.GetWorkflowEventCapabilities()[event]
	allowed := make(map[project_model.WorkflowActionType]bool, len(caps.AvailableActions))
	for _, at := range caps.AvailableActions {
		allowed[at] = true
	}

	for key, value := range formActions {
		actionType := project_model.WorkflowActionType(key)
		if !allowed[actionType] {
			continue // not supported for this event
		}
		switch actionType {
		case project_model.WorkflowActionTypeColumn:
			if colValue, ok := value.(string); ok {
				colValueInt, _ := strconv.ParseInt(colValue, 10, 64)
				if colValueInt > 0 {
					col, _ := project_model.GetColumnByIDAndProjectID(ctx, colValueInt, project.ID)
					if col == nil {
						continue
					}
					actions = append(actions, project_model.WorkflowAction{
						Type:  project_model.WorkflowActionTypeColumn,
						Value: strconv.FormatInt(colValueInt, 10),
					})
				}
			}
		case project_model.WorkflowActionTypeAddLabels:
			// Handle both []string and []any from JSON unmarshaling
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				for _, label := range labels {
					if label != "" {
						labelID, _ := strconv.ParseInt(label, 10, 64)
						if project_service.CanProjectAddLabel(ctx, project, labelID) {
							actions = append(actions, project_model.WorkflowAction{
								Type:  project_model.WorkflowActionTypeAddLabels,
								Value: label,
							})
						}
					}
				}
			} else if labelInterfaces, ok := value.([]any); ok && len(labelInterfaces) > 0 {
				for _, labelInterface := range labelInterfaces {
					if label, ok := labelInterface.(string); ok && label != "" {
						labelID, _ := strconv.ParseInt(label, 10, 64)
						if !project_service.CanProjectAddLabel(ctx, project, labelID) {
							continue
						}
						actions = append(actions, project_model.WorkflowAction{
							Type:  project_model.WorkflowActionTypeAddLabels,
							Value: label,
						})
					}
				}
			}
		case project_model.WorkflowActionTypeRemoveLabels:
			// Handle both []string and []any from JSON unmarshaling
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				for _, label := range labels {
					if label != "" {
						labelID, _ := strconv.ParseInt(label, 10, 64)
						if !project_service.CanProjectAddLabel(ctx, project, labelID) {
							continue
						}
						actions = append(actions, project_model.WorkflowAction{
							Type:  project_model.WorkflowActionTypeRemoveLabels,
							Value: label,
						})
					}
				}
			} else if labelInterfaces, ok := value.([]any); ok && len(labelInterfaces) > 0 {
				for _, labelInterface := range labelInterfaces {
					if label, ok := labelInterface.(string); ok && label != "" {
						labelID, _ := strconv.ParseInt(label, 10, 64)
						if !project_service.CanProjectAddLabel(ctx, project, labelID) {
							continue
						}
						actions = append(actions, project_model.WorkflowAction{
							Type:  project_model.WorkflowActionTypeRemoveLabels,
							Value: label,
						})
					}
				}
			}
		case project_model.WorkflowActionTypeIssueState:
			if strValue, ok := value.(string); ok {
				v := strings.ToLower(strValue)
				if v == "close" || v == "reopen" {
					actions = append(actions, project_model.WorkflowAction{
						Type:  project_model.WorkflowActionTypeIssueState,
						Value: v,
					})
				}
			}
		}
	}

	return actions
}

type WorkflowConfig struct {
	ID            int64                                   `json:"id"`
	EventID       string                                  `json:"event_id"`
	DisplayName   string                                  `json:"display_name"`
	WorkflowEvent string                                  `json:"workflow_event"` // The workflow event
	Capabilities  project_model.WorkflowEventCapabilities `json:"capabilities"`
	Filters       []project_model.WorkflowFilter          `json:"filters"`
	Actions       []project_model.WorkflowAction          `json:"actions"`
	Summary       string                                  `json:"summary"` // Human readable filter description
	Enabled       bool                                    `json:"enabled"`
	IsConfigured  bool                                    `json:"is_configured"` // Whether this workflow is configured/saved
}

func renderWorkflowsEvents(ctx *context.Context, project *project_model.Project) {
	workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
	if err != nil {
		ctx.ServerError("FindWorkflowsByProjectID", err)
		return
	}

	outputWorkflows := make([]*WorkflowConfig, 0)
	events := project_model.GetWorkflowEvents()
	capabilities := project_model.GetWorkflowEventCapabilities()

	// Create a map for quick lookup of existing workflows
	workflowMap := make(map[project_model.WorkflowEvent][]*project_model.Workflow)
	for _, wf := range workflows {
		workflowMap[wf.WorkflowEvent] = append(workflowMap[wf.WorkflowEvent], wf)
	}

	for _, event := range events {
		existingWorkflows := workflowMap[event]
		if len(existingWorkflows) > 0 {
			// Add all existing workflows for this event
			for _, wf := range existingWorkflows {
				workflowSummary := project_service.GetWorkflowSummary(ctx, wf)
				outputWorkflows = append(outputWorkflows, &WorkflowConfig{
					ID:            wf.ID,
					EventID:       strconv.FormatInt(wf.ID, 10),
					DisplayName:   string(ctx.Tr(wf.WorkflowEvent.LangKey())),
					WorkflowEvent: string(wf.WorkflowEvent),
					Capabilities:  capabilities[event],
					Filters:       wf.WorkflowFilters,
					Actions:       wf.WorkflowActions,
					Summary:       workflowSummary,
					Enabled:       wf.Enabled,
					IsConfigured:  true,
				})
			}
		} else {
			// Add placeholder for creating new workflow
			outputWorkflows = append(outputWorkflows, &WorkflowConfig{
				ID:            0,
				EventID:       event.EventID(),
				DisplayName:   string(ctx.Tr(event.LangKey())),
				WorkflowEvent: string(event),
				Capabilities:  capabilities[event],
				Summary:       "",
				Enabled:       true, // Default to enabled for new workflows
				IsConfigured:  false,
			})
		}
	}

	ctx.JSON(http.StatusOK, outputWorkflows)
}

func renderWorkflowsOptions(ctx *context.Context, project *project_model.Project) {
	columns, err := project.GetColumns(ctx)
	if err != nil {
		ctx.ServerError("GetProjectColumns", err)
		return
	}

	outputColumns := make([]*api.ProjectWorkflowColumnOption, 0, len(columns))
	for _, col := range columns {
		outputColumns = append(outputColumns, &api.ProjectWorkflowColumnOption{
			ID:    col.ID,
			Title: col.Title,
			Color: col.Color,
		})
	}

	labels, err := project_service.GetProjectLabels(ctx, project)
	if err != nil {
		ctx.ServerError("GetProjectLabels", err)
		return
	}

	outputLabels := make([]*api.Label, 0, len(labels))
	for _, label := range labels {
		outputLabels = append(outputLabels, &api.Label{
			ID:             label.ID,
			Name:           label.Name,
			Color:          label.Color,
			Description:    label.Description,
			Exclusive:      label.Exclusive,
			ExclusiveOrder: label.ExclusiveOrder,
		})
	}

	ctx.JSON(http.StatusOK, api.ProjectWorkflowOptions{
		Columns: outputColumns,
		Labels:  outputLabels,
	})
}

func prepareProject(ctx *context.Context) *project_model.Project {
	projectID := ctx.PathParamInt64("id")
	p, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return nil
	}
	if p.Type == project_model.TypeRepository && p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return nil
	}
	if (p.Type == project_model.TypeOrganization || p.Type == project_model.TypeIndividual) && p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound(nil)
		return nil
	}
	return p
}

func WorkflowsEvents(ctx *context.Context) {
	p := prepareProject(ctx)
	if p == nil {
		return
	}

	renderWorkflowsEvents(ctx, p)
}

func WorkflowsOptions(ctx *context.Context) {
	p := prepareProject(ctx)
	if p == nil {
		return
	}

	renderWorkflowsOptions(ctx, p)
}

func Workflows(ctx *context.Context) {
	p := prepareProject(ctx)
	if p == nil {
		return
	}

	workflowIDStr := ctx.PathParam("workflow_id")

	ctx.Data["WorkflowEvents"] = project_model.GetWorkflowEvents()

	ctx.Data["Title"] = ctx.Tr("projects.workflows")
	ctx.Data["IsProjectsPage"] = true
	ctx.Data["Project"] = p
	ctx.Data["CanWriteProjects"] = canWriteProjectWorkflows(ctx, p)

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, p.ID)
	if err != nil {
		ctx.ServerError("FindWorkflowsByProjectID", err)
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
			if curWorkflow == nil {
				ctx.NotFound(nil)
				return
			}
		}
	}
	ctx.Data["CurWorkflow"] = curWorkflow
	ctx.Data["ProjectLink"] = project_model.ProjectLinkForRepo(ctx.Repo.Repository, p.ID)

	if p.Type == project_model.TypeRepository {
		ctx.HTML(http.StatusOK, tmplRepoWorkflows)
	} else {
		ctx.HTML(http.StatusOK, tmplOrgWorkflows)
	}
}

func canWriteProjectWorkflows(ctx *context.Context, project *project_model.Project) bool {
	if project.Type == project_model.TypeRepository {
		return ctx.Repo.Permission.CanWrite(unit.TypeProjects)
	}
	if ctx.ContextUser != nil && ctx.ContextUser.IsOrganization() {
		return ctx.Org.CanWriteUnit(ctx, unit.TypeProjects)
	}
	return ctx.Doer != nil && ctx.ContextUser != nil && ctx.ContextUser.ID == ctx.Doer.ID
}

type WorkflowsPostForm struct {
	EventID string         `json:"event_id"`
	Filters map[string]any `json:"filters"`
	Actions map[string]any `json:"actions"`
}

// WorkflowsPost handles creating or updating a workflow
func WorkflowsPost(ctx *context.Context) {
	p := prepareProject(ctx)
	if p == nil {
		return
	}

	form := &WorkflowsPostForm{}
	content, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.ServerError("ReadRequestBody", err)
		return
	}
	defer ctx.Req.Body.Close()
	if err := json.Unmarshal(content, &form); err != nil {
		ctx.ServerError("DecodeWorkflowsPostForm", err)
		return
	}
	if form.EventID == "" {
		ctx.JSONError("EventID is required")
		return
	}

	// Determine the workflow event before converting filters/actions so we can
	// validate against the event's capabilities.
	eventID, _ := strconv.ParseInt(form.EventID, 10, 64)
	var (
		workflowEvent project_model.WorkflowEvent
		existingWf    *project_model.Workflow
	)
	if eventID == 0 {
		if !project_model.IsValidWorkflowEvent(form.EventID) {
			ctx.JSONError(fmt.Sprintf("EventID %s is invalid", form.EventID))
			return
		}
		workflowEvent = project_model.WorkflowEvent(form.EventID)
	} else {
		existingWf, err = project_model.GetWorkflowByProjectAndID(ctx, p.ID, eventID)
		if err != nil {
			if db.IsErrNotExist(err) {
				ctx.NotFound(nil)
			} else {
				ctx.ServerError("GetWorkflowByID", err)
			}
			return
		}
		workflowEvent = existingWf.WorkflowEvent
	}

	// Convert and validate filters/actions against the event's capabilities.
	filters := convertFormToFilters(ctx, p, workflowEvent, form.Filters)
	actions := convertFormToActions(ctx, p, workflowEvent, form.Actions)

	if len(actions) == 0 {
		ctx.JSONError(ctx.Tr("projects.workflows.at_least_one_action_required"))
		return
	}

	if existingWf == nil {
		// Create a new workflow for the given event.
		wf := &project_model.Workflow{
			ProjectID:       p.ID,
			WorkflowEvent:   workflowEvent,
			WorkflowFilters: filters,
			WorkflowActions: actions,
			Enabled:         true,
		}
		if err := project_model.CreateWorkflow(ctx, wf); err != nil {
			ctx.ServerError("CreateWorkflow", err)
			return
		}
		workflowSummary := project_service.GetWorkflowSummary(ctx, wf)
		ctx.JSON(http.StatusOK, map[string]any{
			"success": true,
			"workflow": WorkflowConfig{
				ID:            wf.ID,
				EventID:       strconv.FormatInt(wf.ID, 10),
				DisplayName:   string(ctx.Tr(wf.WorkflowEvent.LangKey())),
				WorkflowEvent: string(wf.WorkflowEvent),
				Capabilities:  project_model.GetWorkflowEventCapabilities()[wf.WorkflowEvent],
				Filters:       wf.WorkflowFilters,
				Actions:       wf.WorkflowActions,
				Summary:       workflowSummary,
				Enabled:       wf.Enabled,
				IsConfigured:  true,
			},
		})
		return
	}

	// Update the existing workflow.
	existingWf.WorkflowFilters = filters
	existingWf.WorkflowActions = actions
	if err := project_model.UpdateWorkflow(ctx, existingWf); err != nil {
		ctx.ServerError("UpdateWorkflow", err)
		return
	}
	workflowSummary := project_service.GetWorkflowSummary(ctx, existingWf)
	ctx.JSON(http.StatusOK, map[string]any{
		"success": true,
		"workflow": WorkflowConfig{
			ID:            existingWf.ID,
			EventID:       strconv.FormatInt(existingWf.ID, 10),
			DisplayName:   string(ctx.Tr(existingWf.WorkflowEvent.LangKey())),
			WorkflowEvent: string(existingWf.WorkflowEvent),
			Capabilities:  project_model.GetWorkflowEventCapabilities()[existingWf.WorkflowEvent],
			Filters:       existingWf.WorkflowFilters,
			Actions:       existingWf.WorkflowActions,
			Summary:       workflowSummary,
			Enabled:       existingWf.Enabled,
			IsConfigured:  true,
		},
	})
}

func WorkflowsStatus(ctx *context.Context) {
	p := prepareProject(ctx)
	if p == nil {
		return
	}

	workflowID := ctx.PathParamInt64("workflow_id")
	_, err := project_model.GetWorkflowByProjectAndID(ctx, p.ID, workflowID)
	if err != nil {
		if db.IsErrNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetWorkflowByID", err)
		}
		return
	}

	// Get enabled status from form
	_ = ctx.Req.ParseForm()
	enabledStr := ctx.Req.FormValue("enabled")
	enabled, _ := strconv.ParseBool(enabledStr)

	if enabled {
		if err := project_model.EnableWorkflow(ctx, workflowID); err != nil {
			ctx.ServerError("EnableWorkflow", err)
			return
		}
	} else {
		if err := project_model.DisableWorkflow(ctx, workflowID); err != nil {
			ctx.ServerError("DisableWorkflow", err)
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"success": true,
		"enabled": enabled,
	})
}

func WorkflowsDelete(ctx *context.Context) {
	p := prepareProject(ctx)
	if p == nil {
		return
	}

	workflowID := ctx.PathParamInt64("workflow_id")
	wf, err := project_model.GetWorkflowByProjectAndID(ctx, p.ID, workflowID)
	if err != nil {
		if db.IsErrNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetWorkflowByID", err)
		}
		return
	}

	if err := project_model.DeleteWorkflow(ctx, wf.ID); err != nil {
		ctx.ServerError("DeleteWorkflow", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"success": true,
	})
}
