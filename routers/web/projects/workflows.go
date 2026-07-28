// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"gitea.dev/models/db"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unit"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	project_service "gitea.dev/services/projects"
)

var (
	tmplRepoWorkflows = templates.TplName("repo/projects/workflows")
	tmplOrgWorkflows  = templates.TplName("org/projects/workflows")
)

// sortedFormKeys returns the keys of a JSON-decoded form object in a stable order, so that
// filters/actions built from a map[string]any are stored (and diffable) in a deterministic order.
func sortedFormKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// extractFormStringList converts a JSON-decoded value into its non-empty string entries.
// encoding/json only ever decodes a JSON array into []any (never []string), so that's the
// only shape handled here; anything else yields no entries.
func extractFormStringList(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			result = append(result, s)
		}
	}
	return result
}

// validateFormLabels reports ok=false, after writing a JSON error to the response, if any of
// the given label IDs is not one the project can use: silently dropping it would broaden the
// filter/action to match every label instead of the ones the user picked.
func validateFormLabels(ctx *context.Context, project *project_model.Project, labels []string) (validated []string, ok bool) {
	for _, label := range labels {
		labelID, err := strconv.ParseInt(label, 10, 64)
		if err != nil || labelID <= 0 || !project_service.CanProjectAddLabel(ctx, project, labelID) {
			ctx.JSONError("invalid label: " + label)
			return nil, false
		}
		validated = append(validated, label)
	}
	return validated, true
}

// convertFormToFilters converts form filters to WorkflowFilter objects. It reports ok=false,
// after writing a JSON error to the response, if a filter value cannot be resolved: silently
// dropping it would broaden the workflow to match everything the user meant to filter on.
func convertFormToFilters(ctx *context.Context, project *project_model.Project, event project_model.WorkflowEvent, formFilters map[string]any) (filters []project_model.WorkflowFilter, ok bool) {
	filters = make([]project_model.WorkflowFilter, 0)

	caps := project_model.GetWorkflowEventCapabilities()[event]
	allowed := make(map[project_model.WorkflowFilterType]bool, len(caps.AvailableFilters))
	for _, ft := range caps.AvailableFilters {
		allowed[ft] = true
	}

	for _, key := range sortedFormKeys(formFilters) {
		value := formFilters[key]
		filterType := project_model.WorkflowFilterType(key)
		if !allowed[filterType] {
			continue // not supported for this event
		}
		switch filterType {
		case project_model.WorkflowFilterTypeLabels:
			labels, labelsOK := validateFormLabels(ctx, project, extractFormStringList(value))
			if !labelsOK {
				return nil, false
			}
			for _, label := range labels {
				filters = append(filters, project_model.WorkflowFilter{Type: filterType, Value: label})
			}
		case project_model.WorkflowFilterTypeSourceColumn, project_model.WorkflowFilterTypeTargetColumn:
			strValue, _ := value.(string)
			if strValue == "" {
				continue // not set, this filter is simply absent
			}
			columnID, err := strconv.ParseInt(strValue, 10, 64)
			if err != nil || columnID <= 0 {
				ctx.JSONError("invalid " + string(filterType) + ": " + strValue)
				return nil, false
			}
			col, _ := project_model.GetColumnByIDAndProjectID(ctx, columnID, project.ID)
			if col == nil {
				ctx.JSONError("invalid " + string(filterType) + ": " + strValue)
				return nil, false
			}
			filters = append(filters, project_model.WorkflowFilter{
				Type:  filterType,
				Value: strconv.FormatInt(columnID, 10),
			})
		case project_model.WorkflowFilterTypeIssueType:
			strValue, _ := value.(string)
			if strValue == "" {
				continue // not set, this filter is simply absent
			}
			// an unknown value would match every item instead of filtering, so reject it
			if !project_model.IsValidWorkflowIssueType(strValue) {
				ctx.JSONError("invalid issue_type: " + strValue)
				return nil, false
			}
			filters = append(filters, project_model.WorkflowFilter{
				Type:  filterType,
				Value: strValue,
			})
		}
	}

	return filters, true
}

// convertFormToActions converts form actions to WorkflowAction objects. It reports ok=false,
// after writing a JSON error to the response, if an action value cannot be resolved: silently
// dropping it would leave the workflow doing less than the user configured, or nothing at all.
func convertFormToActions(ctx *context.Context, project *project_model.Project, event project_model.WorkflowEvent, formActions map[string]any) (actions []project_model.WorkflowAction, ok bool) {
	actions = make([]project_model.WorkflowAction, 0)

	caps := project_model.GetWorkflowEventCapabilities()[event]
	allowed := make(map[project_model.WorkflowActionType]bool, len(caps.AvailableActions))
	for _, at := range caps.AvailableActions {
		allowed[at] = true
	}

	for _, key := range sortedFormKeys(formActions) {
		value := formActions[key]
		actionType := project_model.WorkflowActionType(key)
		if !allowed[actionType] {
			continue // not supported for this event
		}
		switch actionType {
		case project_model.WorkflowActionTypeColumn:
			strValue, _ := value.(string)
			if strValue == "" {
				continue // not set, this action is simply absent
			}
			columnID, err := strconv.ParseInt(strValue, 10, 64)
			if err != nil || columnID <= 0 {
				ctx.JSONError("invalid column: " + strValue)
				return nil, false
			}
			col, _ := project_model.GetColumnByIDAndProjectID(ctx, columnID, project.ID)
			if col == nil {
				ctx.JSONError("invalid column: " + strValue)
				return nil, false
			}
			actions = append(actions, project_model.WorkflowAction{
				Type:  project_model.WorkflowActionTypeColumn,
				Value: strconv.FormatInt(columnID, 10),
			})
		case project_model.WorkflowActionTypeAddLabels, project_model.WorkflowActionTypeRemoveLabels:
			labels, labelsOK := validateFormLabels(ctx, project, extractFormStringList(value))
			if !labelsOK {
				return nil, false
			}
			for _, label := range labels {
				actions = append(actions, project_model.WorkflowAction{Type: actionType, Value: label})
			}
		case project_model.WorkflowActionTypeIssueState:
			strValue, _ := value.(string)
			if strValue == "" {
				continue // not set, this action is simply absent
			}
			issueState := strings.ToLower(strValue)
			if issueState != "close" && issueState != "reopen" {
				ctx.JSONError("invalid issue_state: " + strValue)
				return nil, false
			}
			actions = append(actions, project_model.WorkflowAction{
				Type:  project_model.WorkflowActionTypeIssueState,
				Value: issueState,
			})
		}
	}

	return actions, true
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
		})
	}

	labels, err := project_service.GetProjectLabels(ctx, project)
	if err != nil {
		ctx.ServerError("GetProjectLabels", err)
		return
	}

	// use the shared converter (also used by the API options endpoint) so both surfaces
	// emit the identical label representation, e.g. color without the leading '#'.
	// ctx.Repo.Repository is nil for org/user-scoped projects, but ToLabel only
	// dereferences it for repo-owned labels, none of which GetProjectLabels returns in
	// that case; ctx.ContextUser is always the correct owner for either scope (it's set
	// to ctx.Repo.Owner on repo routes, and to the org/user itself on owner routes).
	outputLabels := convert.ToLabelList(labels, ctx.Repo.Repository, ctx.ContextUser)

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
	// the project scope must match the ROUTE, not just the project's own owner: on a repo route
	// context.RepoAssignment sets ctx.ContextUser = ctx.Repo.Owner, so checking only p.OwnerID would
	// let a repo route serve an unrelated org/user-level project owned by the repo's owner.
	if ctx.Repo.Repository != nil {
		if p.Type != project_model.TypeRepository || p.RepoID != ctx.Repo.Repository.ID {
			ctx.NotFound(nil)
			return nil
		}
	} else if p.Type == project_model.TypeRepository || ctx.ContextUser == nil || p.OwnerID != ctx.ContextUser.ID {
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
	ctx.Data["ProjectLink"] = p.Link(ctx)

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
	filters, ok := convertFormToFilters(ctx, p, workflowEvent, form.Filters)
	if !ok {
		return
	}
	actions, ok := convertFormToActions(ctx, p, workflowEvent, form.Actions)
	if !ok {
		return
	}

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
	// the frontend always sends the stringified JS boolean ("true"/"false"); reject anything
	// else explicitly instead of silently falling back to "disabled" on a malformed value
	enabled, err := strconv.ParseBool(enabledStr)
	if err != nil {
		ctx.JSONError("invalid enabled: " + enabledStr)
		return
	}

	if enabled {
		if err := project_model.EnableWorkflow(ctx, p.ID, workflowID); err != nil {
			ctx.ServerError("EnableWorkflow", err)
			return
		}
	} else {
		if err := project_model.DisableWorkflow(ctx, p.ID, workflowID); err != nil {
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

	if err := project_model.DeleteWorkflow(ctx, p.ID, wf.ID); err != nil {
		ctx.ServerError("DeleteWorkflow", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"success": true,
	})
}
