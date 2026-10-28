// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

type WorkflowEvent string

const (
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

type WorkflowFilters struct {
	IssueType      string  `json:"issue_type,omitempty"`
	SourceColumnID int64   `json:"source_column_id,omitempty"`
	TargetColumnID int64   `json:"target_column_id,omitempty"`
	LabelIDs       []int64 `json:"label_ids,omitempty"`
}

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

type WorkflowEventCapabilities struct {
	Filters []WorkflowFilterType
	Actions []WorkflowActionType
}

var (
	itemFilters = []WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeLabels}
	pullFilters = []WorkflowFilterType{WorkflowFilterTypeLabels}
	pullActions = []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels}
)

var workflowEvents = []struct {
	event   WorkflowEvent
	filters []WorkflowFilterType
	actions []WorkflowActionType
}{
	{WorkflowEventItemAddedToProject, itemFilters, []WorkflowActionType{WorkflowActionTypeColumn, WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState}},
	{WorkflowEventItemRemovedFromProject, itemFilters, []WorkflowActionType{WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState}},
	{WorkflowEventItemReopened, itemFilters, pullActions},
	{WorkflowEventItemClosed, itemFilters, pullActions},
	{
		WorkflowEventItemColumnChanged,
		[]WorkflowFilterType{WorkflowFilterTypeIssueType, WorkflowFilterTypeSourceColumn, WorkflowFilterTypeTargetColumn, WorkflowFilterTypeLabels},
		[]WorkflowActionType{WorkflowActionTypeAddLabels, WorkflowActionTypeRemoveLabels, WorkflowActionTypeIssueState},
	},
	{WorkflowEventCodeChangesRequested, pullFilters, pullActions},
	{WorkflowEventCodeReviewApproved, pullFilters, pullActions},
	{WorkflowEventPullRequestMerged, pullFilters, pullActions},
}

func WorkflowEvents() []WorkflowEvent {
	events := make([]WorkflowEvent, len(workflowEvents))
	for i, event := range workflowEvents {
		events[i] = event.event
	}
	return events
}

func (we WorkflowEvent) IsValid() bool {
	return slices.Contains(WorkflowEvents(), we)
}

func (we WorkflowEvent) Capabilities() WorkflowEventCapabilities {
	for _, event := range workflowEvents {
		if event.event == we {
			return WorkflowEventCapabilities{Filters: event.filters, Actions: event.actions}
		}
	}
	return WorkflowEventCapabilities{}
}

func (we WorkflowEvent) LangKey() string {
	return "projects.workflows.event." + string(we)
}

type Workflow struct {
	ID              int64 `xorm:"pk autoincr"`
	ProjectID       int64 `xorm:"INDEX"`
	WorkflowEvent   WorkflowEvent
	WorkflowFilters WorkflowFilters    `xorm:"TEXT JSON"`
	WorkflowActions WorkflowActions    `xorm:"TEXT JSON"`
	Enabled         bool               `xorm:"DEFAULT true NOT NULL"`
	CreatedUnix     timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
}

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

type FindWorkflowsOptions struct {
	db.ListOptions
	ProjectIDs []int64
	Event      WorkflowEvent
	Enabled    optional.Option[bool]
}

func (opts FindWorkflowsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if len(opts.ProjectIDs) > 0 {
		cond = cond.And(builder.In("project_id", opts.ProjectIDs))
	}
	if opts.Event != "" {
		cond = cond.And(builder.Eq{"workflow_event": opts.Event})
	}
	if opts.Enabled.Has() {
		cond = cond.And(builder.Eq{"enabled": opts.Enabled.Value()})
	}
	return cond
}

func (opts FindWorkflowsOptions) ToOrders() string {
	return "project_id ASC, id ASC" // keep execution order stable across database engines
}

// FindEnabledWorkflows returns the workflows of open projects among projectIDs that can hold items of repo, see CanBeAccessedByOwnerRepo
func FindEnabledWorkflows(ctx context.Context, repo *repo_model.Repository, projectIDs []int64, event WorkflowEvent) ([]*Workflow, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}
	opts := FindWorkflowsOptions{ProjectIDs: projectIDs, Event: event, Enabled: optional.Some(true)}
	holdsRepoItems := builder.Eq{"repo_id": repo.ID}.Or(builder.Eq{"owner_id": repo.OwnerID, "repo_id": 0})
	workflows := make([]*Workflow, 0)
	err := db.GetEngine(ctx).Where(opts.ToConds()).
		In("project_id", builder.Select("id").From("project").Where(builder.Eq{"is_closed": false}.And(holdsRepoItems))).
		OrderBy(opts.ToOrders()).Find(&workflows)
	return workflows, err
}

func GetWorkflowByProjectAndID(ctx context.Context, projectID, workflowID int64) (*Workflow, error) {
	workflow, exist, err := db.Get[Workflow](ctx, builder.Eq{"project_id": projectID, "id": workflowID})
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, db.ErrNotExist{Resource: "ProjectWorkflow", ID: workflowID}
	}
	return workflow, nil
}

func CreateWorkflow(ctx context.Context, wf *Workflow) error {
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
