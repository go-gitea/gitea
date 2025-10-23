// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

type WorkflowEvent string

const (
	WorkflowEventItemOpened           WorkflowEvent = "item_opened"
	WorkflowEventItemAddedToProject   WorkflowEvent = "item_added_to_project"
	WorkflowEventItemReopened         WorkflowEvent = "item_reopened"
	WorkflowEventItemClosed           WorkflowEvent = "item_closed"
	WorkflowEventItemColumnChanged    WorkflowEvent = "item_column_changed"
	WorkflowEventCodeChangesRequested WorkflowEvent = "code_changes_requested"
	WorkflowEventCodeReviewApproved   WorkflowEvent = "code_review_approved"
	WorkflowEventPullRequestMerged    WorkflowEvent = "pull_request_merged"
)

var workflowEvents = []WorkflowEvent{
	WorkflowEventItemOpened,
	WorkflowEventItemAddedToProject,
	WorkflowEventItemReopened,
	WorkflowEventItemClosed,
	WorkflowEventItemColumnChanged,
	WorkflowEventCodeChangesRequested,
	WorkflowEventCodeReviewApproved,
	WorkflowEventPullRequestMerged,
}

func GetWorkflowEvents() []WorkflowEvent {
	return workflowEvents
}

func (we WorkflowEvent) LangKey() string {
	switch we {
	case WorkflowEventItemOpened:
		return "projects.workflows.event.item_opened"
	case WorkflowEventItemAddedToProject:
		return "projects.workflows.event.item_added_to_project"
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

func (we WorkflowEvent) UUID() string {
	switch we {
	case WorkflowEventItemOpened:
		return "item_opened"
	case WorkflowEventItemAddedToProject:
		return "item_added_to_project"
	case WorkflowEventItemReopened:
		return "item_reopened"
	case WorkflowEventItemClosed:
		return "item_closed"
	case WorkflowEventItemColumnChanged:
		return "item_column_changed"
	case WorkflowEventCodeChangesRequested:
		return "code_changes_requested"
	case WorkflowEventCodeReviewApproved:
		return "code_review_approved"
	case WorkflowEventPullRequestMerged:
		return "pull_request_merged"
	default:
		return string(we)
	}
}

type WorkflowFilterType string

const (
	WorkflowFilterTypeIssueType WorkflowFilterType = "issue_type" // issue, pull_request, etc.
	WorkflowFilterTypeColumn    WorkflowFilterType = "column"     // target column for item_column_changed event
)

type WorkflowFilter struct {
	Type  WorkflowFilterType `json:"type"`
	Value string             `json:"value"`
}

type WorkflowActionType string

const (
	WorkflowActionTypeColumn       WorkflowActionType = "column"        // add the item to the project's column
	WorkflowActionTypeAddLabels    WorkflowActionType = "add_labels"    // choose one or more labels
	WorkflowActionTypeRemoveLabels WorkflowActionType = "remove_labels" // choose one or more labels
	WorkflowActionTypeClose        WorkflowActionType = "close"         // close the issue
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
func GetWorkflowEventCapabilities() map[WorkflowEvent]WorkflowEventCapabilities {
	return map[WorkflowEvent]WorkflowEventCapabilities{
		WorkflowEventItemOpened: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType}, // issue, pull_request
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels},
		},
		WorkflowEventItemAddedToProject: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType}, // issue, pull_request
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventItemReopened: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventItemClosed: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventItemColumnChanged: {
			AvailableFilters: []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeColumn},
			AvailableActions: []WorkflowActionType{WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeClose},
		},
		WorkflowEventCodeChangesRequested: {
			AvailableFilters: []WorkflowFilterType{}, // only applies to pull requests
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventCodeReviewApproved: {
			AvailableFilters: []WorkflowFilterType{}, // only applies to pull requests
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
		WorkflowEventPullRequestMerged: {
			AvailableFilters: []WorkflowFilterType{}, // only applies to pull requests
			AvailableActions: []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels},
		},
	}
}

type Workflow struct {
	ID              int64
	ProjectID       int64              `xorm:"INDEX"`
	Project         *Project           `xorm:"-"`
	WorkflowEvent   WorkflowEvent      `xorm:"INDEX"`
	WorkflowFilters []WorkflowFilter   `xorm:"TEXT json"`
	WorkflowActions []WorkflowAction   `xorm:"TEXT json"`
	Enabled         bool               `xorm:"DEFAULT true"`
	CreatedUnix     timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
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
	if err := db.GetEngine(ctx).Where("project_id=?", projectID).Find(&workflows); err != nil {
		return nil, err
	}
	return workflows, nil
}

func GetWorkflowByID(ctx context.Context, id int64) (*Workflow, error) {
	p, exist, err := db.GetByID[Workflow](ctx, id)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, util.ErrNotExist
	}
	return p, nil
}

func CreateWorkflow(ctx context.Context, wf *Workflow) error {
	return db.Insert(ctx, wf)
}

func UpdateWorkflow(ctx context.Context, wf *Workflow) error {
	_, err := db.GetEngine(ctx).ID(wf.ID).Cols("workflow_filters", "workflow_actions").Update(wf)
	return err
}

func DeleteWorkflow(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&Workflow{})
	return err
}

func EnableWorkflow(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Cols("enabled").Update(&Workflow{Enabled: true})
	return err
}

func DisableWorkflow(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Cols("enabled").Update(&Workflow{Enabled: false})
	return err
}
