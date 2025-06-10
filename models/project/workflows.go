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

var workflowEvents = []WorkflowEvent{
	WorkflowEventItemAddedToProject,
	WorkflowEventItemReopened,
	WorkflowEventItemClosed,
	WorkflowEventCodeChangesRequested,
	WorkflowEventCodeReviewApproved,
	WorkflowEventPullRequestMerged,
	WorkflowEventAutoArchiveItems,
	WorkflowEventAutoAddToProject,
	WorkflowEventAutoCloseIssue,
}

func GetWorkflowEvents() []WorkflowEvent {
	return workflowEvents
}

func (we WorkflowEvent) ToString() string {
	switch we {
	case WorkflowEventItemAddedToProject:
		return "Item added to project"
	case WorkflowEventItemReopened:
		return "Item reopened"
	case WorkflowEventItemClosed:
		return "Item closed"
	case WorkflowEventCodeChangesRequested:
		return "Code changes requested"
	case WorkflowEventCodeReviewApproved:
		return "Code review approved"
	case WorkflowEventPullRequestMerged:
		return "Pull request merged"
	case WorkflowEventAutoArchiveItems:
		return "Auto archive items"
	case WorkflowEventAutoAddToProject:
		return "Auto add to project"
	case WorkflowEventAutoCloseIssue:
		return "Auto close issue"
	default:
		return string(we)
	}
}

func (we WorkflowEvent) UUID() string {
	switch we {
	case WorkflowEventItemAddedToProject:
		return "item_added_to_project"
	case WorkflowEventItemReopened:
		return "item_reopened"
	case WorkflowEventItemClosed:
		return "item_closed"
	case WorkflowEventCodeChangesRequested:
		return "code_changes_requested"
	case WorkflowEventCodeReviewApproved:
		return "code_review_approved"
	case WorkflowEventPullRequestMerged:
		return "pull_request_merged"
	case WorkflowEventAutoArchiveItems:
		return "auto_archive_items"
	case WorkflowEventAutoAddToProject:
		return "auto_add_to_project"
	case WorkflowEventAutoCloseIssue:
		return "auto_close_issue"
	default:
		return string(we)
	}
}

type WorkflowFilterType string

const (
	WorkflowFilterTypeScope WorkflowFilterType = "scope" // issue, pull_request, etc.
)

type WorkflowFilter struct {
	Type  WorkflowFilterType
	Value string // e.g., "issue", "pull_request", etc.
}

type WorkflowActionType string

const (
	WorkflowActionTypeColumn WorkflowActionType = "column" // add the item to the project's column
	WorkflowActionTypeLabel  WorkflowActionType = "label"  // choose one or more labels
	WorkflowActionTypeClose  WorkflowActionType = "close"  // close the issue
)

type WorkflowAction struct {
	ActionType  WorkflowActionType
	ActionValue string
}

type ProjectWorkflow struct {
	ID              int64
	ProjectID       int64              `xorm:"unique(s)"`
	Project         *Project           `xorm:"-"`
	WorkflowEvent   WorkflowEvent      `xorm:"unique(s)"`
	WorkflowFilters []WorkflowFilter   `xorm:"TEXT json"`
	WorkflowActions []WorkflowAction   `xorm:"TEXT json"`
	CreatedUnix     timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
}

func (p *ProjectWorkflow) LoadProject(ctx context.Context) error {
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

func (p *ProjectWorkflow) Link(ctx context.Context) string {
	if err := p.LoadProject(ctx); err != nil {
		log.Error("ProjectWorkflow Link: %v", err)
		return ""
	}
	return p.Project.Link(ctx) + fmt.Sprintf("/workflows/%d", p.ID)
}

func init() {
	db.RegisterModel(new(ProjectWorkflow))
}

func FindWorkflowEvents(ctx context.Context, projectID int64) (map[WorkflowEvent]*ProjectWorkflow, error) {
	events := make(map[WorkflowEvent]*ProjectWorkflow)
	if err := db.GetEngine(ctx).Where("project_id=?", projectID).Find(&events); err != nil {
		return nil, err
	}
	res := make(map[WorkflowEvent]*ProjectWorkflow, len(events))
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
