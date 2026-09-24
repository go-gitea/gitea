// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"
	"sync"

	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
)

type WorkflowEvent string

const (
	WorkflowEventItemOpened             WorkflowEvent = "item_opened"
	WorkflowEventItemAddedToProject     WorkflowEvent = "item_added_to_project"
	WorkflowEventItemRemovedFromProject WorkflowEvent = "item_removed_from_project"
	WorkflowEventItemReopened           WorkflowEvent = "item_reopened"
	WorkflowEventItemClosed             WorkflowEvent = "item_closed"
	WorkflowEventItemColumnChanged      WorkflowEvent = "item_column_changed"
	WorkflowEventCodeChangesRequested   WorkflowEvent = "code_changes_requested"
	WorkflowEventCodeReviewApproved     WorkflowEvent = "code_review_approved"
	WorkflowEventPullRequestMerged      WorkflowEvent = "pull_request_merged"
)

var GetWorkflowEvents = sync.OnceValue(func() []WorkflowEvent {
	return []WorkflowEvent{
		WorkflowEventItemOpened,
		WorkflowEventItemAddedToProject,
		WorkflowEventItemRemovedFromProject,
		WorkflowEventItemReopened,
		WorkflowEventItemClosed,
		WorkflowEventItemColumnChanged,
		WorkflowEventCodeChangesRequested,
		WorkflowEventCodeReviewApproved,
		WorkflowEventPullRequestMerged,
	}
})

func IsValidWorkflowEvent(event string) bool {
	for _, we := range GetWorkflowEvents() {
		if we.EventID() == event {
			return true
		}
	}
	return false
}

func (we WorkflowEvent) LangKey() string {
	switch we {
	case WorkflowEventItemOpened:
		return "projects.workflows.event.item_opened"
	case WorkflowEventItemAddedToProject:
		return "projects.workflows.event.item_added_to_project"
	case WorkflowEventItemRemovedFromProject:
		return "projects.workflows.event.item_removed_from_project"
	case WorkflowEventItemReopened:
		return "projects.workflows.event.item_reopened"
	case WorkflowEventItemClosed:
		return "projects.workflows.event.item_closed"
	case WorkflowEventItemColumnChanged:
		return "projects.workflows.event.item_column_changed"
	case WorkflowEventCodeChangesRequested:
		return "projects.workflows.event.code_changes_requested"
	case WorkflowEventCodeReviewApproved:
		return "projects.workflows.event.code_review_approved"
	case WorkflowEventPullRequestMerged:
		return "projects.workflows.event.pull_request_merged"
	default:
		return string(we)
	}
}

func (we WorkflowEvent) EventID() string {
	return string(we)
}

type WorkflowFilterType string

const (
	WorkflowFilterTypeIssueType    WorkflowFilterType = "issue_type"    // issue, pull_request, etc.
	WorkflowFilterTypeSourceColumn WorkflowFilterType = "source_column" // source column for item_column_changed event
	WorkflowFilterTypeTargetColumn WorkflowFilterType = "target_column" // target column for item_column_changed event
	WorkflowFilterTypeLabels       WorkflowFilterType = "labels"        // filter by issue/PR labels
)

// accepted values of the WorkflowFilterTypeIssueType filter
const (
	WorkflowIssueTypeIssue       = "issue"
	WorkflowIssueTypePullRequest = "pull_request"
)

func IsValidWorkflowIssueType(issueType string) bool {
	return issueType == WorkflowIssueTypeIssue || issueType == WorkflowIssueTypePullRequest
}

type WorkflowFilter struct {
	Type  WorkflowFilterType `json:"type"`
	Value string             `json:"value"`
}

type WorkflowActionType string

const (
	WorkflowActionTypeColumn       WorkflowActionType = "column"        // add the item to the project's column
	WorkflowActionTypeAddLabels    WorkflowActionType = "add_labels"    // choose one or more labels
	WorkflowActionTypeRemoveLabels WorkflowActionType = "remove_labels" // choose one or more labels
	WorkflowActionTypeIssueState   WorkflowActionType = "issue_state"   // change the issue state (reopen/close)
)

type WorkflowAction struct {
	Type  WorkflowActionType `json:"type"`
	Value string             `json:"value"`
}

// WorkflowEventCapabilities defines what filters and actions are available for each event
type WorkflowEventCapabilities struct {
	AvailableFilters []WorkflowFilterType `json:"available_filters"`
	AvailableActions []WorkflowActionType `json:"available_actions"`
}

// GetWorkflowEventCapabilities returns the capabilities for each workflow event
var GetWorkflowEventCapabilities = sync.OnceValue(func() map[WorkflowEvent]WorkflowEventCapabilities {
	return map[WorkflowEvent]WorkflowEventCapabilities{
		WorkflowEventItemOpened: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeLabels},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels},
		},
		WorkflowEventItemAddedToProject: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeLabels},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState},
		},
		WorkflowEventItemRemovedFromProject: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeLabels},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState},
		},
		WorkflowEventItemReopened: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeLabels},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventItemClosed: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeLabels},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventItemColumnChanged: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeSourceColumn, WorkflowFilterTypeTargetColumn, WorkflowFilterTypeLabels},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState},
		},
		WorkflowEventCodeChangesRequested: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeLabels}, // only applies to pull requests
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventCodeReviewApproved: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeLabels}, // only applies to pull requests
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventPullRequestMerged: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeLabels}, // only applies to pull requests
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
	}
})

type Workflow struct {
	ID              int64
	ProjectID       int64    `xorm:"INDEX"`
	Project         *Project `xorm:"-"`
	WorkflowEvent   WorkflowEvent
	WorkflowFilters []WorkflowFilter `xorm:"TEXT JSON"`
	WorkflowActions []WorkflowAction `xorm:"TEXT JSON"`
	// SchemaVersion number to allow for smooth version upgrades of WorkflowFilters/
	// WorkflowActions, following the same rationale as HookTask.PayloadVersion:
	//  - SchemaVersion 1: WorkflowFilters/WorkflowActions use the current filter/action shape
	SchemaVersion int                `xorm:"DEFAULT 1"`
	Enabled       bool               `xorm:"DEFAULT true NOT NULL"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
}

// TableName overrides the table name used by ProjectWorkflow to `project_workflow`
func (Workflow) TableName() string {
	return "project_workflow"
}

func (p *Workflow) LoadProject(ctx context.Context) error {
	if p.Project != nil || p.ProjectID <= 0 {
		return nil
	}
	project, err := GetProjectByID(ctx, p.ProjectID)
	if err != nil {
		return err
	}
	p.Project = project
	return nil
}

func (p *Workflow) Link(ctx context.Context) string {
	if err := p.LoadProject(ctx); err != nil {
		log.Error("ProjectWorkflow Link: %v", err)
		return ""
	}
	return p.Project.Link(ctx) + fmt.Sprintf("/workflows/%d", p.ID)
}

func init() {
	db.RegisterModel(new(Workflow))
}

func FindWorkflowsByProjectID(ctx context.Context, projectID int64) ([]*Workflow, error) {
	workflows := make([]*Workflow, 0)
	// Explicit ORDER BY: without it the row order is engine-dependent (e.g. on
	// Postgres an UPDATE can relocate a row), so toggling Enabled could silently
	// reorder the workflows that run for the same event.
	if err := db.GetEngine(ctx).Where("project_id=?", projectID).OrderBy("id ASC").Find(&workflows); err != nil {
		return nil, err
	}
	return workflows, nil
}

func GetWorkflowByProjectAndID(ctx context.Context, projectID, workflowID int64) (*Workflow, error) {
	var workflow Workflow
	exist, err := db.GetEngine(ctx).Where("project_id=? AND id=?", projectID, workflowID).Get(&workflow)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, db.ErrNotExist{Resource: "ProjectWorkflow", ID: workflowID}
	}
	return &workflow, nil
}

// CurrentWorkflowSchemaVersion is written to new workflows; see Workflow.SchemaVersion.
const CurrentWorkflowSchemaVersion = 1

func CreateWorkflow(ctx context.Context, wf *Workflow) error {
	// callers don't set SchemaVersion themselves (there is only one shape today),
	// so default it here rather than relying on the DB column default, which
	// xorm's Insert would otherwise bypass by sending the Go zero value literally.
	if wf.SchemaVersion == 0 {
		wf.SchemaVersion = CurrentWorkflowSchemaVersion
	}
	return db.Insert(ctx, wf)
}

// the mutators below are all scoped by project_id so that a workflow ID from
// another project can never be modified, even if a caller forgets to check

func UpdateWorkflow(ctx context.Context, wf *Workflow) error {
	_, err := db.GetEngine(ctx).ID(wf.ID).Where("project_id=?", wf.ProjectID).
		Cols("workflow_filters", "workflow_actions").Update(wf)
	return err
}

func DeleteWorkflow(ctx context.Context, projectID, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Where("project_id=?", projectID).Delete(&Workflow{})
	return err
}

func EnableWorkflow(ctx context.Context, projectID, id int64) error {
	return setWorkflowEnabled(ctx, projectID, id, true)
}

func DisableWorkflow(ctx context.Context, projectID, id int64) error {
	return setWorkflowEnabled(ctx, projectID, id, false)
}

func setWorkflowEnabled(ctx context.Context, projectID, id int64, enabled bool) error {
	_, err := db.GetEngine(ctx).ID(id).Where("project_id=?", projectID).
		Cols("enabled").Update(&Workflow{Enabled: enabled})
	return err
}
