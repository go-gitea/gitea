// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

// WorkflowEvent is an event on an issue or pull request that can move its card
type WorkflowEvent string

const (
	WorkflowEventItemAdded         WorkflowEvent = "item_added"
	WorkflowEventItemClosed        WorkflowEvent = "item_closed"
	WorkflowEventItemReopened      WorkflowEvent = "item_reopened"
	WorkflowEventPullRequestMerged WorkflowEvent = "pull_request_merged"
	WorkflowEventChangesRequested  WorkflowEvent = "changes_requested"
	WorkflowEventReviewApproved    WorkflowEvent = "review_approved"
	// WorkflowEventCardMoved fires when a card is moved into the target column, and closes its issue
	WorkflowEventCardMoved WorkflowEvent = "card_moved"
)

// WorkflowEvents lists the supported events in display order
var WorkflowEvents = []WorkflowEvent{
	WorkflowEventItemAdded,
	WorkflowEventItemReopened,
	WorkflowEventItemClosed,
	WorkflowEventPullRequestMerged,
	WorkflowEventChangesRequested,
	WorkflowEventReviewApproved,
	WorkflowEventCardMoved,
}

func (e WorkflowEvent) IsValid() bool {
	return slices.Contains(WorkflowEvents, e)
}

// Workflow is a per-project rule: when Event happens to an item, its card goes to the target column
type Workflow struct {
	ID        int64         `xorm:"pk autoincr"`
	ProjectID int64         `xorm:"UNIQUE(project_event) NOT NULL"`
	Event     WorkflowEvent `xorm:"UNIQUE(project_event) VARCHAR(32) NOT NULL"`
	ColumnID  int64         `xorm:"NOT NULL"`
	Enabled   bool          `xorm:"NOT NULL DEFAULT false"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func (Workflow) TableName() string {
	return "project_workflow"
}

func init() {
	db.RegisterModel(new(Workflow))
}

// GetWorkflowsByProjectID returns the workflows configured for a project, keyed by event
func GetWorkflowsByProjectID(ctx context.Context, projectID int64) (map[WorkflowEvent]*Workflow, error) {
	workflows := make([]*Workflow, 0, len(WorkflowEvents))
	if err := db.GetEngine(ctx).Where("project_id=?", projectID).Find(&workflows); err != nil {
		return nil, err
	}
	m := make(map[WorkflowEvent]*Workflow, len(workflows))
	for _, w := range workflows {
		m[w.Event] = w
	}
	return m, nil
}

// GetEnabledWorkflow returns the enabled workflow of a project for an event, or nil if there is none
func GetEnabledWorkflow(ctx context.Context, projectID int64, event WorkflowEvent) (*Workflow, error) {
	w := &Workflow{}
	has, err := db.GetEngine(ctx).Where("project_id=? AND event=? AND enabled=?", projectID, event, true).Get(w)
	if err != nil || !has {
		return nil, err
	}
	return w, nil
}

// GetEnabledWorkflows returns the enabled workflows for an event across several projects, keyed by project ID
func GetEnabledWorkflows(ctx context.Context, projectIDs []int64, event WorkflowEvent) (map[int64]*Workflow, error) {
	if len(projectIDs) == 0 {
		return map[int64]*Workflow{}, nil
	}
	workflows := make([]*Workflow, 0, len(projectIDs))
	if err := db.GetEngine(ctx).In("project_id", projectIDs).And("event=? AND enabled=?", event, true).Find(&workflows); err != nil {
		return nil, err
	}
	m := make(map[int64]*Workflow, len(workflows))
	for _, w := range workflows {
		m[w.ProjectID] = w
	}
	return m, nil
}

// SaveWorkflow creates or updates the workflow of a project for an event
func SaveWorkflow(ctx context.Context, projectID int64, event WorkflowEvent, columnID int64, enabled bool) error {
	if !event.IsValid() {
		return util.NewInvalidArgumentErrorf("invalid workflow event %q", event)
	}
	column, err := GetColumnByIDAndProjectID(ctx, columnID, projectID)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		existing := &Workflow{}
		has, err := db.GetEngine(ctx).Where("project_id=? AND event=?", projectID, event).Get(existing)
		if err != nil {
			return err
		}
		if !has {
			return db.Insert(ctx, &Workflow{ProjectID: projectID, Event: event, ColumnID: column.ID, Enabled: enabled})
		}
		existing.ColumnID, existing.Enabled = column.ID, enabled
		_, err = db.GetEngine(ctx).ID(existing.ID).Cols("column_id", "enabled").Update(existing)
		return err
	})
}

// GetColumnForNewItem returns the column a newly added item lands in: the target of the
// "item added" workflow when enabled, otherwise the default column
func (p *Project) GetColumnForNewItem(ctx context.Context) (*Column, error) {
	w, err := GetEnabledWorkflow(ctx, p.ID, WorkflowEventItemAdded)
	if err != nil {
		return nil, err
	}
	if w != nil {
		if column, err := GetColumnByIDAndProjectID(ctx, w.ColumnID, p.ID); err == nil {
			return column, nil
		} else if !IsErrProjectColumnNotExist(err) {
			return nil, err
		}
	}
	return p.MustDefaultColumn(ctx)
}

func deleteWorkflowsByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&Workflow{})
	return err
}

func deleteWorkflowsByColumnID(ctx context.Context, columnID int64) error {
	_, err := db.GetEngine(ctx).Where("column_id=?", columnID).Delete(&Workflow{})
	return err
}
