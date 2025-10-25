// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	stdCtx "context"
	"io"
	"net/http"
	"strconv"
	"strings"

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
func getFilterSummary(ctx stdCtx.Context, filters []project_model.WorkflowFilter) string {
	if len(filters) == 0 {
		return ""
	}

	var summary strings.Builder
	labelIDs := make([]int64, 0)
	for _, filter := range filters {
		switch filter.Type {
		case project_model.WorkflowFilterTypeIssueType:
			switch filter.Value {
			case "issue":
				if summary.Len() > 0 {
					summary.WriteString(" ")
				}
				summary.WriteString("(Issues only)")
			case "pull_request":
				if summary.Len() > 0 {
					summary.WriteString(" ")
				}
				summary.WriteString("(Pull requests only)")
			}
		case project_model.WorkflowFilterTypeSourceColumn:
			columnID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if columnID <= 0 {
				continue
			}
			col, err := project_model.GetColumn(ctx, columnID)
			if err != nil {
				log.Error("GetColumn: %v", err)
				continue
			}
			if summary.Len() > 0 {
				summary.WriteString(" ")
			}
			summary.WriteString("(Source: " + col.Title + ")")
		case project_model.WorkflowFilterTypeTargetColumn:
			columnID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if columnID <= 0 {
				continue
			}
			col, err := project_model.GetColumn(ctx, columnID)
			if err != nil {
				log.Error("GetColumn: %v", err)
				continue
			}
			if summary.Len() > 0 {
				summary.WriteString(" ")
			}
			summary.WriteString("(Target: " + col.Title + ")")
		case project_model.WorkflowFilterTypeLabels:
			labelID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if labelID > 0 {
				labelIDs = append(labelIDs, labelID)
			}
		}
	}
	if len(labelIDs) > 0 {
		labels, err := issues_model.GetLabelsByIDs(ctx, labelIDs)
		if err != nil {
			log.Error("GetLabelsByIDs: %v", err)
		} else {
			if summary.Len() > 0 {
				summary.WriteString(" ")
			}
			summary.WriteString("(Labels: ")
			for i, label := range labels {
				summary.WriteString(label.Name)
				if i < len(labels)-1 {
					summary.WriteString(", ")
				}
			}
			summary.WriteString(")")
		}
	}
	return summary.String()
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
		case "issue_state":
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
		ID            int64                                   `json:"id"`
		EventID       string                                  `json:"event_id"`
		DisplayName   string                                  `json:"display_name"`
		WorkflowEvent string                                  `json:"workflow_event"` // The workflow event
		Capabilities  project_model.WorkflowEventCapabilities `json:"capabilities"`
		Filters       []project_model.WorkflowFilter          `json:"filters"`
		Actions       []project_model.WorkflowAction          `json:"actions"`
		FilterSummary string                                  `json:"filter_summary"` // Human readable filter description
		Enabled       bool                                    `json:"enabled"`
		IsConfigured  bool                                    `json:"isConfigured"` // Whether this workflow is configured/saved
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
				filterSummary := getFilterSummary(ctx, wf.WorkflowFilters)
				outputWorkflows = append(outputWorkflows, &WorkflowConfig{
					ID:            wf.ID,
					EventID:       strconv.FormatInt(wf.ID, 10),
					DisplayName:   string(ctx.Tr(wf.WorkflowEvent.LangKey())),
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
				EventID:       event.EventID(),
				DisplayName:   string(ctx.Tr(event.LangKey())),
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
		Color string `json:"color"`
	}
	outputColumns := make([]*Column, 0, len(columns))
	for _, col := range columns {
		outputColumns = append(outputColumns, &Column{
			ID:    col.ID,
			Title: col.Title,
			Color: col.Color,
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

// WorkflowsPost handles creating or updating a workflow
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
		ctx.JSON(http.StatusBadRequest, map[string]any{"error": "InvalidEventID", "message": "EventID is required"})
		return
	}

	// Convert form data to filters and actions
	filters := convertFormToFilters(form.Filters)
	actions := convertFormToActions(form.Actions)

	eventID, _ := strconv.ParseInt(form.EventID, 10, 64)
	if eventID == 0 {
		// check if workflow event is valid
		if !project_model.IsValidWorkflowEvent(form.EventID) {
			ctx.JSON(http.StatusBadRequest, map[string]any{"error": "EventID is invalid"})
			return
		}

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
		filterSummary := getFilterSummary(ctx, wf.WorkflowFilters)
		ctx.JSON(http.StatusOK, map[string]any{
			"success": true,
			"workflow": map[string]any{
				"id":             wf.ID,
				"event_id":       strconv.FormatInt(wf.ID, 10),
				"display_name":   string(ctx.Tr(wf.WorkflowEvent.LangKey())),
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
		filterSummary := getFilterSummary(ctx, wf.WorkflowFilters)
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
		if db.IsErrNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetWorkflowByID", err)
		}
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
