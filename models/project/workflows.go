// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

type WorkflowEvent string

const (
	WorkflowEventItemAddedToProject   WorkflowEvent = "item_added_to_project"
	WorkflowEventItemReopened         WorkflowEvent = "item_reopened"
	WorkflowEventItemClosed           WorkflowEvent = "item_closed"
	WorkflowEventCodeChangesRequested WorkflowEvent = "code_changes_requested"
	WorkflowEventCodeReviewApproved   WorkflowEvent = "code_review_approved"
	WorkflowEventPullRequestMerged    WorkflowEvent = "pull_request_merged"
	WorkflowEventAutoArchiveItems     WorkflowEvent = "auto_archive_items"
	WorkflowEventAutoAddToProject     WorkflowEvent = "auto_add_to_project"
	WorkflowEventAutoCloseIssue       WorkflowEvent = "auto_close_issue"
)

type WorkflowActionType string

const (
	WorkflowActionTypeScope  WorkflowActionType = "scope"  // issue, pull_request, etc.
	WorkflowActionTypeLabel  WorkflowActionType = "label"  // choose one or more labels
	WorkflowActionTypeColumn WorkflowActionType = "column" // choose one column
	WorkflowActionTypeClose  WorkflowActionType = "close"  // close the issue
)

type WorkflowAction struct {
	ActionType  WorkflowActionType
	ActionValue string
}

type ProjectWorkflow struct {
	ID              int64
	ProjectID       int64              `xorm:"unique(s)"`
	WorkflowEvent   WorkflowEvent      `xorm:"unique(s)"`
	WorkflowActions []WorkflowAction   `xorm:"TEXT json"`
	CreatedUnix     timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
}

func (p *ProjectWorkflow) Link() string {
	return ""
}

func newDefaultWorkflows() []*ProjectWorkflow {
	return []*ProjectWorkflow{
		{
			WorkflowEvent:   WorkflowEventItemAddedToProject,
			WorkflowActions: []WorkflowAction{{ActionType: WorkflowActionTypeScope, ActionValue: "issue"}},
		},
		{
			ProjectID:       0,
			WorkflowEvent:   WorkflowEventItemReopened,
			WorkflowActions: []WorkflowAction{{ActionType: WorkflowActionTypeScope, ActionValue: "issue"}},
		},
	}
}

func GetWorkflowDefaultValue(workflowIDStr string) *ProjectWorkflow {
	workflows := newDefaultWorkflows()
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == WorkflowEvent(workflowIDStr) {
			return workflow
		}
	}
	return &ProjectWorkflow{}
}

func init() {
	db.RegisterModel(new(ProjectWorkflow))
}

func FindWorkflowEvents(ctx context.Context, projectID int64) (map[WorkflowEvent]ProjectWorkflow, error) {
	events := make(map[WorkflowEvent]ProjectWorkflow)
	if err := db.GetEngine(ctx).Where("project_id=?", projectID).Find(&events); err != nil {
		return nil, err
	}
	res := make(map[WorkflowEvent]ProjectWorkflow, len(events))
	for _, event := range events {
		res[event.WorkflowEvent] = event
	}
	return res, nil
}

func GetWorkflowByID(ctx context.Context, id int64) (*ProjectWorkflow, error) {
	p, exist, err := db.GetByID[ProjectWorkflow](ctx, id)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, util.ErrNotExist
	}
	return p, nil
}

func GetWorkflows(ctx context.Context, projectID int64) ([]*ProjectWorkflow, error) {
	events := make([]*ProjectWorkflow, 0, 10)
	if err := db.GetEngine(ctx).Where("project_id=?", projectID).Find(&events); err != nil {
		return nil, err
	}
	workflows := newDefaultWorkflows()
	for i, defaultWorkflow := range workflows {
		for _, workflow := range events {
			if workflow.WorkflowEvent == defaultWorkflow.WorkflowEvent {
				workflows[i] = workflow
			}
		}
	}
	return workflows, nil
}
