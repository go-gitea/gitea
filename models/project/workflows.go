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

type ProjectWorkflowEvent struct {
	ID              int64
	ProjectID       int64              `xorm:"unique(s)"`
	WorkflowEvent   WorkflowEvent      `xorm:"unique(s)"`
	WorkflowActions []WorkflowAction   `xorm:"TEXT json"`
	CreatedUnix     timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ProjectWorkflowEvent))
}

func FindWorkflowEvents(ctx context.Context, projectID int64) (map[WorkflowEvent]ProjectWorkflowEvent, error) {
	events := make(map[WorkflowEvent]ProjectWorkflowEvent)
	if err := db.GetEngine(ctx).Where("project_id=?", projectID).Find(&events); err != nil {
		return nil, err
	}
	res := make(map[WorkflowEvent]ProjectWorkflowEvent, len(events))
	for _, event := range events {
		res[event.WorkflowEvent] = event
	}
	return res, nil
}

func GetWorkflowEventByID(ctx context.Context, id int64) (*ProjectWorkflowEvent, error) {
	p, exist, err := db.GetByID[ProjectWorkflowEvent](ctx, id)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, util.ErrNotExist
	}
	return p, nil
}
