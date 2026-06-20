// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"strconv"
	"strings"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	issue_service "gitea.dev/services/issue"
	notify_service "gitea.dev/services/notify"
)

func init() {
	notify_service.RegisterNotifier(&workflowNotifier{})
}

type workflowNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &workflowNotifier{}

// NewNotifier create a new workflowNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &workflowNotifier{}
}

func (m *workflowNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("NewIssue: LoadRepo: %v", err)
		return
	}
	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("NewIssue: LoadProject: %v", err)
		return
	}
	if len(issue.Projects) == 0 {
		return
	}

	for _, project := range issue.Projects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
		if err != nil {
			log.Error("NewIssue: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the ItemOpened event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == project_model.WorkflowEventItemOpened {
				fireIssueWorkflow(ctx, workflow, issue, project.ID, 0, 0)
			}
		}
	}
}

func (m *workflowNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("NewPullRequest: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue
	m.NewIssue(ctx, issue, mentions)
}

func (m *workflowNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	// Skip state changes triggered by workflow actions to prevent cascade loops
	// (same guard as feed/notifier.go).
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadRepo: %v", err)
		return
	}
	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadProject: %v", err)
		return
	}
	if len(issue.Projects) == 0 {
		return
	}

	for _, project := range issue.Projects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
		if err != nil {
			log.Error("IssueChangeStatus: FindWorkflowsByProjectID: %v", err)
			return
		}

		workflowEvent := util.Iif(isClosed, project_model.WorkflowEventItemClosed, project_model.WorkflowEventItemReopened)
		// Find workflows for the specific event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == workflowEvent {
				fireIssueWorkflow(ctx, workflow, issue, project.ID, 0, 0)
			}
		}
	}
}

func (*workflowNotifier) IssueChangeProjects(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldProjectColumnMap map[int64]int64, newProjects []*project_model.Project) {
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	addedProjects := make(map[int64]*project_model.Project)
	for _, newProject := range newProjects {
		// Use key presence check; column ID 0 is technically valid.
		if _, ok := oldProjectColumnMap[newProject.ID]; ok {
			continue
		}
		addedProjects[newProject.ID] = newProject
	}
	var removedProjectIDs []int64
	for projectID := range oldProjectColumnMap {
		found := false
		for _, newProject := range newProjects {
			if newProject.ID == projectID {
				found = true
				break
			}
		}
		if !found {
			removedProjectIDs = append(removedProjectIDs, projectID)
		}
	}

	for _, removedProjectID := range removedProjectIDs {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, removedProjectID)
		if err != nil {
			log.Error("IssueChangeProjects: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the ItemRemovedFromProject event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == project_model.WorkflowEventItemRemovedFromProject {
				fireIssueWorkflow(ctx, workflow, issue, removedProjectID, 0, 0)
			}
		}
	}

	for _, newProject := range addedProjects {
		if err := issue.LoadRepo(ctx); err != nil {
			log.Error("IssueChangeProjects: LoadRepo: %v", err)
			return
		}

		workflows, err := project_model.FindWorkflowsByProjectID(ctx, newProject.ID)
		if err != nil {
			log.Error("IssueChangeProjects: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the ItemOpened event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == project_model.WorkflowEventItemAddedToProject {
				fireIssueWorkflow(ctx, workflow, issue, newProject.ID, 0, 0)
			}
		}
	}
}

func (*workflowNotifier) IssueChangeProjectColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldColumnID, newColumnID int64) {
	// Skip column moves triggered by workflow actions to prevent cascade loops.
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeProjectColumn: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("IssueChangeProjectColumn: LoadProjects: %v", err)
		return
	}

	oldColumn, err := project_model.GetColumn(ctx, oldColumnID)
	if err != nil {
		log.Error("IssueChangeProjectColumn: GetColumn: %v", err)
		return
	}

	newColumn, err := project_model.GetColumn(ctx, newColumnID)
	if err != nil {
		log.Error("IssueChangeProjectColumn: GetColumn: %v", err)
		return
	}
	if oldColumn.ProjectID != newColumn.ProjectID {
		return
	}
	found := false
	for _, project := range issue.Projects {
		if project.ID == oldColumn.ProjectID {
			found = true
			break
		}
	}
	if !found {
		return
	}

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, oldColumn.ProjectID)
	if err != nil {
		log.Error("IssueChangeProjectColumn: FindWorkflowsByProjectID: %v", err)
		return
	}

	// Find workflows for the ItemColumnChanged event
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == project_model.WorkflowEventItemColumnChanged {
			fireIssueWorkflow(ctx, workflow, issue, oldColumn.ProjectID, oldColumnID, newColumnID)
		}
	}
}

func (*workflowNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("MergePullRequest: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("MergePullRequest: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("MergePullRequest: LoadProjects: %v", err)
		return
	}
	if len(issue.Projects) == 0 {
		return
	}

	for _, project := range issue.Projects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
		if err != nil {
			log.Error("MergePullRequest: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the PullRequestMerged event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == project_model.WorkflowEventPullRequestMerged {
				fireIssueWorkflow(ctx, workflow, issue, project.ID, 0, 0)
			}
		}
	}
}

func (m *workflowNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	m.MergePullRequest(ctx, doer, pr)
}

func (*workflowNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("PullRequestReview: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("PullRequestReview: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("PullRequestReview: LoadProjects: %v", err)
		return
	}
	if len(issue.Projects) == 0 {
		return
	}

	for _, project := range issue.Projects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
		if err != nil {
			log.Error("PullRequestReview: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the PullRequestMerged event
		for _, workflow := range workflows {
			if (workflow.WorkflowEvent == project_model.WorkflowEventCodeChangesRequested && review.Type == issues_model.ReviewTypeReject) ||
				(workflow.WorkflowEvent == project_model.WorkflowEventCodeReviewApproved && review.Type == issues_model.ReviewTypeApprove) {
				fireIssueWorkflow(ctx, workflow, issue, project.ID, 0, 0)
			}
		}
	}
}

func fireIssueWorkflow(ctx context.Context, workflow *project_model.Workflow, issue *issues_model.Issue, projectID, sourceColumnID, targetColumnID int64) {
	if !workflow.Enabled {
		return
	}

	// Load issue labels for labels filter
	if err := issue.LoadLabels(ctx); err != nil {
		log.Error("LoadLabels: %v", err)
		return
	}

	if !matchWorkflowsFilters(workflow, issue, sourceColumnID, targetColumnID) {
		return
	}

	executeWorkflowActions(ctx, workflow, issue, projectID)
}

// matchWorkflowsFilters checks if the issue matches all filters of the workflow
func matchWorkflowsFilters(workflow *project_model.Workflow, issue *issues_model.Issue, sourceColumnID, targetColumnID int64) bool {
	for _, filter := range workflow.WorkflowFilters {
		switch filter.Type {
		case project_model.WorkflowFilterTypeIssueType:
			// If filter value is empty, match all types
			if filter.Value == "" {
				continue
			}
			// Filter value can be "issue" or "pull_request"
			if filter.Value == "issue" && issue.IsPull {
				return false
			}
			if filter.Value == "pull_request" && !issue.IsPull {
				return false
			}
		case project_model.WorkflowFilterTypeTargetColumn:
			// If filter value is empty, match all columns
			if filter.Value == "" {
				continue
			}
			filterColumnID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if filterColumnID == 0 {
				log.Error("Invalid column ID: %s", filter.Value)
				return false
			}
			// For column changed event, check against the new column ID
			if targetColumnID > 0 && targetColumnID != filterColumnID {
				return false
			}
		case project_model.WorkflowFilterTypeSourceColumn:
			// If filter value is empty, match all columns
			if filter.Value == "" {
				continue
			}
			filterColumnID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if filterColumnID == 0 {
				log.Error("Invalid column ID: %s", filter.Value)
				return false
			}
			// For column changed event, check against the new column ID
			if sourceColumnID > 0 && sourceColumnID != filterColumnID {
				return false
			}
		case project_model.WorkflowFilterTypeLabels:
			// Check if issue has the specified label
			labelID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if labelID == 0 {
				log.Error("Invalid label ID: %s", filter.Value)
				return false
			}
			// Check if issue has this label
			hasLabel := false
			for _, label := range issue.Labels {
				if label.ID == labelID {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				return false
			}
		default:
			log.Error("Unsupported filter type: %s", filter.Type)
			return false
		}
	}
	return true
}

func executeWorkflowActions(ctx context.Context, workflow *project_model.Workflow, issue *issues_model.Issue, projectID int64) {
	if err := workflow.LoadProject(ctx); err != nil {
		log.Error("LoadProject: %v", err)
	}

	title := "(untitled project)"
	if workflow.Project != nil {
		title = workflow.Project.Title
	}

	doer := issues_model.NewProjectWorkflowDoer(title, workflow.ID, workflow.WorkflowEvent)
	var toAddedLabels, toRemovedLabels []*issues_model.Label

	for _, action := range workflow.WorkflowActions {
		switch action.Type {
		case project_model.WorkflowActionTypeColumn:
			columnID, _ := strconv.ParseInt(action.Value, 10, 64)
			if columnID == 0 {
				log.Error("Invalid column ID: %s", action.Value)
				continue
			}
			column, err := project_model.GetColumnByIDAndProjectID(ctx, columnID, projectID)
			if err != nil {
				log.Error("GetColumnByIDAndProjectID: %v", err)
				continue
			}
			if err := MoveIssueToAnotherColumn(ctx, doer, issue, column); err != nil {
				log.Error("MoveIssueToAnotherColumn: %v", err)
				continue
			}
		case project_model.WorkflowActionTypeAddLabels:
			labelID, _ := strconv.ParseInt(action.Value, 10, 64)
			if labelID == 0 {
				log.Error("Invalid label ID: %s", action.Value)
				continue
			}
			label, err := issues_model.GetLabelByID(ctx, labelID)
			if err != nil {
				log.Error("GetLabelByID: %v", err)
				continue
			}
			toAddedLabels = append(toAddedLabels, label)
		case project_model.WorkflowActionTypeRemoveLabels:
			labelID, _ := strconv.ParseInt(action.Value, 10, 64)
			if labelID == 0 {
				log.Error("Invalid label ID: %s", action.Value)
				continue
			}
			label, err := issues_model.GetLabelByID(ctx, labelID)
			if err != nil {
				log.Error("GetLabelByID: %v", err)
				continue
			}
			toRemovedLabels = append(toRemovedLabels, label)
		case project_model.WorkflowActionTypeIssueState:
			if strings.EqualFold(action.Value, "reopen") {
				if issue.IsClosed {
					if err := issue_service.ReopenIssue(ctx, issue, doer, ""); err != nil {
						log.Error("ReopenIssue: %v", err)
						continue
					}
				}
			} else if strings.EqualFold(action.Value, "close") {
				if !issue.IsClosed {
					if err := issue_service.CloseIssue(ctx, issue, doer, ""); err != nil {
						log.Error("CloseIssue: %v", err)
						continue
					}
				}
			}
		default:
			log.Error("Unsupported action type: %s", action.Type)
		}
	}

	if len(toAddedLabels)+len(toRemovedLabels) > 0 {
		if err := issue_service.AddRemoveLabels(ctx, issue, doer, toAddedLabels, toRemovedLabels); err != nil {
			log.Error("AddRemoveLabels: %v", err)
		}
	}
}
