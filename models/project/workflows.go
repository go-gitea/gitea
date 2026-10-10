// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"

	"gitea.dev/models/db"
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

type WorkflowFilterType string

const (
	WorkflowFilterTypeIssueType    WorkflowFilterType = "issue_type"
	WorkflowFilterTypeSourceColumn WorkflowFilterType = "source_column"
	WorkflowFilterTypeTargetColumn WorkflowFilterType = "target_column"
	WorkflowFilterTypeLabels       WorkflowFilterType = "labels"
)

const (
	WorkflowIssueTypeIssue       = "issue"
	WorkflowIssueTypePullRequest = "pull_request"
)

// WorkflowFilters restricts the items a workflow runs for, zero values match everything
type WorkflowFilters struct {
	IssueType      string  `json:"issue_type,omitempty"`
	SourceColumnID int64   `json:"source_column_id,omitempty"`
	TargetColumnID int64   `json:"target_column_id,omitempty"`
	LabelIDs       []int64 `json:"label_ids,omitempty"`
}

// Types returns the types of the filters that are set
func (f WorkflowFilters) Types() (types []WorkflowFilterType) {
	if f.IssueType != "" {
		types = append(types, WorkflowFilterTypeIssueType)
	}
	if f.SourceColumnID != 0 {
		types = append(types, WorkflowFilterTypeSourceColumn)
	}
	if f.TargetColumnID != 0 {
		types = append(types, WorkflowFilterTypeTargetColumn)
	}
	if len(f.LabelIDs) > 0 {
		types = append(types, WorkflowFilterTypeLabels)
	}
	return types
}

type WorkflowActionType string

const (
	WorkflowActionTypeColumn       WorkflowActionType = "column"
	WorkflowActionTypeAddLabels    WorkflowActionType = "add_labels"
	WorkflowActionTypeRemoveLabels WorkflowActionType = "remove_labels"
	WorkflowActionTypeIssueState   WorkflowActionType = "issue_state"
)

const (
	WorkflowIssueStateClose  = "close"
	WorkflowIssueStateReopen = "reopen"
)

type WorkflowActions struct {
	ColumnID       int64   `json:"column_id,omitempty"`
	AddLabelIDs    []int64 `json:"add_label_ids,omitempty"`
	RemoveLabelIDs []int64 `json:"remove_label_ids,omitempty"`
	IssueState     string  `json:"issue_state,omitempty"`
}

// Types returns the types of the actions that are set
func (a WorkflowActions) Types() (types []WorkflowActionType) {
	if a.ColumnID != 0 {
		types = append(types, WorkflowActionTypeColumn)
	}
	if len(a.AddLabelIDs) > 0 {
		types = append(types, WorkflowActionTypeAddLabels)
	}
	if len(a.RemoveLabelIDs) > 0 {
		types = append(types, WorkflowActionTypeRemoveLabels)
	}
	if a.IssueState != "" {
		types = append(types, WorkflowActionTypeIssueState)
	}
	return types
}

// ChangesIssue reports whether the actions edit the item itself rather than only its place on the board
func (a WorkflowActions) ChangesIssue() bool {
	return len(a.AddLabelIDs) > 0 || len(a.RemoveLabelIDs) > 0 || a.IssueState != ""
}

type WorkflowEventCapabilities struct {
	Filters []WorkflowFilterType
	Actions []WorkflowActionType
}

var workflowEvents = []WorkflowEvent{
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

var (
	itemFilters = []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeLabels}
	pullFilters = []WorkflowFilterType{WorkflowFilterTypeLabels}
	allActions  = []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState}
	pullActions = []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels}
)

var workflowEventCapabilities = map[WorkflowEvent]WorkflowEventCapabilities{
	WorkflowEventItemOpened:             {itemFilters, []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels}},
	WorkflowEventItemAddedToProject:     {itemFilters, allActions},
	WorkflowEventItemRemovedFromProject: {itemFilters, []WorkflowActionType{WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState}},
	WorkflowEventItemReopened:           {itemFilters, pullActions},
	WorkflowEventItemClosed:             {itemFilters, pullActions},
	WorkflowEventItemColumnChanged: {
		[]WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeSourceColumn, WorkflowFilterTypeTargetColumn, WorkflowFilterTypeLabels},
		[]WorkflowActionType{WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState},
	},
	WorkflowEventCodeChangesRequested: {pullFilters, pullActions},
	WorkflowEventCodeReviewApproved:   {pullFilters, pullActions},
	WorkflowEventPullRequestMerged:    {pullFilters, pullActions},
}

// WorkflowEvents returns all events in display order
func WorkflowEvents() []WorkflowEvent {
	return workflowEvents
}

func (we WorkflowEvent) IsValid() bool {
	_, ok := workflowEventCapabilities[we]
	return ok
}

func (we WorkflowEvent) Capabilities() WorkflowEventCapabilities {
	return workflowEventCapabilities[we]
}

func (we WorkflowEvent) LangKey() string {
	return "projects.workflows.event." + string(we)
}

type Workflow struct {
	ID              int64
	ProjectID       int64 `xorm:"INDEX"`
	WorkflowEvent   WorkflowEvent
	WorkflowFilters WorkflowFilters `xorm:"TEXT JSON"`
	WorkflowActions WorkflowActions `xorm:"TEXT JSON"`
	// bump when the filter/action JSON shape changes, see HookTask.PayloadVersion
	SchemaVersion int  `xorm:"DEFAULT 1"`
	Enabled       bool `xorm:"DEFAULT true NOT NULL"`
	// label and issue state actions run with this user's permissions
	UpdaterID   int64              `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// UnsupportedRule returns the first filter or action type the workflow's event doesn't support
func (wf *Workflow) UnsupportedRule() string {
	capabilities := wf.WorkflowEvent.Capabilities()
	for _, typ := range wf.WorkflowFilters.Types() {
		if !slices.Contains(capabilities.Filters, typ) {
			return string(typ)
		}
	}
	for _, typ := range wf.WorkflowActions.Types() {
		if !slices.Contains(capabilities.Actions, typ) {
			return string(typ)
		}
	}
	return ""
}

func (Workflow) TableName() string {
	return "project_workflow"
}

func init() {
	db.RegisterModel(new(Workflow))
}

func FindWorkflowsByProjectID(ctx context.Context, projectID int64) ([]*Workflow, error) {
	workflows := make([]*Workflow, 0)
	return workflows, db.GetEngine(ctx).Where("project_id=?", projectID).OrderBy("id ASC").Find(&workflows)
}

func FindEnabledWorkflows(ctx context.Context, projectIDs []int64, event WorkflowEvent) ([]*Workflow, error) {
	workflows := make([]*Workflow, 0)
	if len(projectIDs) == 0 {
		return workflows, nil
	}
	// ordered so execution order is stable across engines
	return workflows, db.GetEngine(ctx).In("project_id", projectIDs).And("workflow_event=? AND enabled=?", event, true).OrderBy("project_id ASC, id ASC").Find(&workflows)
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

const currentWorkflowSchemaVersion = 1

func CreateWorkflow(ctx context.Context, wf *Workflow) error {
	wf.SchemaVersion = currentWorkflowSchemaVersion // xorm inserts the zero value, bypassing the column DEFAULT
	return db.Insert(ctx, wf)
}

func UpdateWorkflow(ctx context.Context, wf *Workflow, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(wf.ID).Where("project_id=?", wf.ProjectID).Cols(cols...).Update(wf)
	return err
}

func DeleteWorkflow(ctx context.Context, projectID, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Where("project_id=?", projectID).Delete(&Workflow{})
	return err
}
