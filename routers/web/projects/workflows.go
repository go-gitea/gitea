// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

var (
	tmplRepoWorkflows = templates.TplName("repo/projects/workflows")
	tmplOrgWorkflows  = templates.TplName("org/projects/workflows")
)

// getFilterSummary returns a human-readable summary of the filters
func getFilterSummary(filters []project_model.WorkflowFilter) string {
	if len(filters) == 0 {
		return ""
	}

	for _, filter := range filters {
		if filter.Type == "scope" {
			switch filter.Value {
			case "issue":
				return " (Issues only)"
			case "pull_request":
				return " (Pull requests only)"
			}
		}
	}
	return ""
}

// convertFormToFilters converts form filters to WorkflowFilter objects
func convertFormToFilters(formFilters map[string]any) []project_model.WorkflowFilter {
	filters := make([]project_model.WorkflowFilter, 0)

	for key, value := range formFilters {
		switch key {
		case "labels":
			// Handle labels array
			if labelInterfaces, ok := value.([]interface{}); ok && len(labelInterfaces) > 0 {
				for _, labelInterface := range labelInterfaces {
					if label, ok := labelInterface.(string); ok && label != "" {
						filters = append(filters, project_model.WorkflowFilter{
							Type:  project_model.WorkflowFilterTypeLabels,
							Value: label,
						})
					}
				}
			}
		default:
			// Handle string values (issue_type, column)
			if strValue, ok := value.(string); ok && strValue != "" {
				filters = append(filters, project_model.WorkflowFilter{
					Type:  project_model.WorkflowFilterType(key),
					Value: strValue,
				})
			}
		}
	}

	return filters
}

// convertFormToActions converts form actions to WorkflowAction objects
func convertFormToActions(formActions map[string]any) []project_model.WorkflowAction {
	actions := make([]project_model.WorkflowAction, 0)

	for key, value := range formActions {
		switch key {
		case "column":
			if floatValue, ok := value.(string); ok {
				floatValueInt, _ := strconv.ParseInt(floatValue, 10, 64)
				if floatValueInt > 0 {
					actions = append(actions, project_model.WorkflowAction{
						Type:  project_model.WorkflowActionTypeColumn,
						Value: strconv.FormatInt(floatValueInt, 10),
					})
				}
			}
		case "add_labels":
			// Handle both []string and []interface{} from JSON unmarshaling
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				for _, label := range labels {
					if label != "" {
						actions = append(actions, project_model.WorkflowAction{
							Type:  project_model.WorkflowActionTypeAddLabels,
							Value: label,
						})
					}
				}
			} else if labelInterfaces, ok := value.([]interface{}); ok && len(labelInterfaces) > 0 {
				for _, labelInterface := range labelInterfaces {
					if label, ok := labelInterface.(string); ok && label != "" {
						actions = append(actions, project_model.WorkflowAction{
							Type:  project_model.WorkflowActionTypeAddLabels,
							Value: label,
						})
					}
				}
			}
		case "remove_labels":
			// Handle both []string and []interface{} from JSON unmarshaling
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				for _, label := range labels {
					if label != "" {
						actions = append(actions, project_model.WorkflowAction{
							Type:  project_model.WorkflowActionTypeRemoveLabels,
							Value: label,
						})
					}
				}
			} else if labelInterfaces, ok := value.([]interface{}); ok && len(labelInterfaces) > 0 {
				for _, labelInterface := range labelInterfaces {
					if label, ok := labelInterface.(string); ok && label != "" {
						actions = append(actions, project_model.WorkflowAction{
							Type:  project_model.WorkflowActionTypeRemoveLabels,
							Value: label,
						})
					}
				}
			}
		case "closeIssue":
			if boolValue, ok := value.(bool); ok && boolValue {
				actions = append(actions, project_model.WorkflowAction{
					Type:  project_model.WorkflowActionTypeClose,
					Value: "true",
				})
			}
		}
	}

	return actions
}

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

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, projectID)
	if err != nil {
		ctx.ServerError("FindWorkflowsByProjectID", err)
		return
	}

	type WorkflowConfig struct {
		ID             int64                                   `json:"id"`
		EventID        string                                  `json:"event_id"`
		DisplayName    string                                  `json:"display_name"`
		BaseEventType  string                                  `json:"base_event_type"`  // Base event type for grouping
		WorkflowEvent  string                                  `json:"workflow_event"`   // The actual workflow event
		Capabilities   project_model.WorkflowEventCapabilities `json:"capabilities"`
		Filters        []project_model.WorkflowFilter          `json:"filters"`
		Actions        []project_model.WorkflowAction          `json:"actions"`
		FilterSummary  string                                  `json:"filter_summary"` // Human readable filter description
		Enabled        bool                                    `json:"enabled"`
		IsConfigured   bool                                    `json:"isConfigured"` // Whether this workflow is configured/saved
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
				filterSummary := getFilterSummary(wf.WorkflowFilters)
				outputWorkflows = append(outputWorkflows, &WorkflowConfig{
					ID:            wf.ID,
					EventID:       strconv.FormatInt(wf.ID, 10),
					DisplayName:   string(ctx.Tr(wf.WorkflowEvent.LangKey())) + filterSummary,
					BaseEventType: string(wf.WorkflowEvent),
					WorkflowEvent: string(wf.WorkflowEvent),
					Capabilities:  capabilities[event],
					Filters:       wf.WorkflowFilters,
					Actions:       wf.WorkflowActions,
					FilterSummary: filterSummary,
					Enabled:       wf.Enabled,
					IsConfigured:  true,
				})
			}
		} else {
			// Add placeholder for creating new workflow
			outputWorkflows = append(outputWorkflows, &WorkflowConfig{
				ID:            0,
				EventID:       event.UUID(),
				DisplayName:   string(ctx.Tr(event.LangKey())),
				BaseEventType: string(event),
				WorkflowEvent: string(event),
				Capabilities:  capabilities[event],
				FilterSummary: "",
				Enabled:       true, // Default to enabled for new workflows
				IsConfigured:  false,
			})
		}
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

func WorkflowsLabels(ctx *context.Context) {
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

	// Only repository projects have access to labels
	if p.Type != project_model.TypeRepository {
		ctx.JSON(http.StatusOK, []any{})
		return
	}

	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}

	// Get repository labels
	labels, err := issues_model.GetLabelsByRepoID(ctx, p.RepoID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}

	type Label struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	outputLabels := make([]*Label, 0, len(labels))
	for _, label := range labels {
		outputLabels = append(outputLabels, &Label{
			ID:    label.ID,
			Name:  label.Name,
			Color: label.Color,
		})
	}

	ctx.JSON(http.StatusOK, outputLabels)
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
	if workflowIDStr == "labels" {
		WorkflowsLabels(ctx)
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
	ctx.Data["IsProjectsPage"] = true
	ctx.Data["Project"] = p

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, projectID)
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
		}
	}
	ctx.Data["CurWorkflow"] = curWorkflow
	ctx.Data["ProjectLink"] = project_model.ProjectLinkForRepo(ctx.Repo.Repository, projectID)

	if p.Type == project_model.TypeRepository {
		ctx.HTML(200, tmplRepoWorkflows)
	} else {
		ctx.HTML(200, tmplOrgWorkflows)
	}
}

type WorkflowsPostForm struct {
	EventID string         `json:"event_id"`
	Filters map[string]any `json:"filters"`
	Actions map[string]any `json:"actions"`
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

	// Handle both form data and JSON data
	// Handle JSON data
	form := &WorkflowsPostForm{}
	content, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.ServerError("ReadRequestBody", err)
		return
	}
	defer ctx.Req.Body.Close()
	log.Trace("get " + string(content))
	if err := json.Unmarshal(content, &form); err != nil {
		ctx.ServerError("DecodeWorkflowsPostForm", err)
		return
	}
	if form.EventID == "" {
		ctx.ServerError("InvalidEventID", errors.New("EventID is required"))
		return
	}

	// Convert form data to filters and actions
	filters := convertFormToFilters(form.Filters)
	actions := convertFormToActions(form.Actions)

	eventID, _ := strconv.ParseInt(form.EventID, 10, 64)
	if eventID == 0 {
		// Create a new workflow for the given event
		wf := &project_model.Workflow{
			ProjectID:       projectID,
			WorkflowEvent:   project_model.WorkflowEvent(form.EventID),
			WorkflowFilters: filters,
			WorkflowActions: actions,
			Enabled:         true, // New workflows are enabled by default
		}
		if err := project_model.CreateWorkflow(ctx, wf); err != nil {
			ctx.ServerError("CreateWorkflow", err)
			return
		}

		// Return the newly created workflow with filter summary
		filterSummary := getFilterSummary(wf.WorkflowFilters)
		ctx.JSON(http.StatusOK, map[string]any{
			"success": true,
			"workflow": map[string]any{
				"id":             wf.ID,
				"event_id":       strconv.FormatInt(wf.ID, 10),
				"display_name":   string(ctx.Tr(wf.WorkflowEvent.LangKey())) + filterSummary,
				"filters":        wf.WorkflowFilters,
				"actions":        wf.WorkflowActions,
				"filter_summary": filterSummary,
				"enabled":        wf.Enabled,
			},
		})
	} else {
		// Update an existing workflow
		wf, err := project_model.GetWorkflowByID(ctx, eventID)
		if err != nil {
			ctx.ServerError("GetWorkflowByID", err)
			return
		}
		if wf.ProjectID != projectID {
			ctx.NotFound(nil)
			return
		}

		wf.WorkflowFilters = filters
		wf.WorkflowActions = actions
		if err := project_model.UpdateWorkflow(ctx, wf); err != nil {
			ctx.ServerError("UpdateWorkflow", err)
			return
		}

		// Return the updated workflow with filter summary
		filterSummary := getFilterSummary(wf.WorkflowFilters)
		ctx.JSON(http.StatusOK, map[string]any{
			"success": true,
			"workflow": map[string]any{
				"id":             wf.ID,
				"event_id":       strconv.FormatInt(wf.ID, 10),
				"display_name":   string(ctx.Tr(wf.WorkflowEvent.LangKey())) + filterSummary,
				"filters":        wf.WorkflowFilters,
				"actions":        wf.WorkflowActions,
				"filter_summary": filterSummary,
				"enabled":        wf.Enabled,
			},
		})
	}
}

func WorkflowsStatus(ctx *context.Context) {
	projectID := ctx.PathParamInt64("id")
	workflowID, _ := strconv.ParseInt(ctx.PathParam("workflow_id"), 10, 64)

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

	wf, err := project_model.GetWorkflowByID(ctx, workflowID)
	if err != nil {
		ctx.ServerError("GetWorkflowByID", err)
		return
	}
	if wf.ProjectID != projectID {
		ctx.NotFound(nil)
		return
	}

	// Get enabled status from form
	enabledStr := ctx.Req.FormValue("enabled")
	enabled := enabledStr == "true"

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
		"enabled": wf.Enabled,
	})
}

func WorkflowsDelete(ctx *context.Context) {
	projectID := ctx.PathParamInt64("id")
	workflowID, _ := strconv.ParseInt(ctx.PathParam("workflow_id"), 10, 64)

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

	wf, err := project_model.GetWorkflowByID(ctx, workflowID)
	if err != nil {
		ctx.ServerError("GetWorkflowByID", err)
		return
	}
	if wf.ProjectID != projectID {
		ctx.NotFound(nil)
		return
	}

	if err := project_model.DeleteWorkflow(ctx, workflowID); err != nil {
		ctx.ServerError("DeleteWorkflow", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"success": true,
	})
}
