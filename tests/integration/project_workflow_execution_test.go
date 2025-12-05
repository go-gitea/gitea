// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testCreateProjectWorkflow(t *testing.T, session *TestSession, userName, repoName string, projectID int64, event string, workflowData map[string]any) {
	req := NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/workflows/%s?_csrf=%s",
			userName, repoName, projectID, event, GetUserCSRFToken(t, session)),
		workflowData)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var result map[string]any
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.True(t, result["success"].(bool))
}

func testNewIssueReturnIssue(t *testing.T, session *TestSession, repo *repo_model.Repository, opts newIssueOptions) int64 {
	testNewIssue(t, session, repo.OwnerName, repo.Name, opts)

	// Get the created issue from database to verify
	issues, err := issues_model.Issues(t.Context(), &issues_model.IssuesOptions{
		RepoIDs:  []int64{repo.ID},
		SortType: "newest",
		Paginator: &db.ListOptions{
			PageSize: 1,
		},
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, issues)
	return issues[0].ID
}

// testAddIssueToProject adds the issue to the project via web form if projectID == 0, it removes the issue from the project
func testAddIssueToProject(t *testing.T, session *TestSession, userName, repoName string, projectID, issueID int64) {
	addToProjectReq := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/issues/projects?_csrf=%s",
		userName, repoName, GetUserCSRFToken(t, session)),
		map[string]string{
			"_csrf":     GetUserCSRFToken(t, session),
			"id":        strconv.FormatInt(projectID, 10),
			"issue_ids": strconv.FormatInt(issueID, 10),
		})
	session.MakeRequest(t, addToProjectReq, http.StatusOK)
}

// TestProjectWorkflowExecutionItemOpened tests workflow execution when an issue is added to project
func TestProjectWorkflowExecutionItemOpened(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create project and columns
	project := &project_model.Project{
		Title:        "Test Workflow Execution",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	assert.NoError(t, project_model.NewProject(t.Context(), project))

	columnToDo := &project_model.Column{
		Title:     "To Do",
		ProjectID: project.ID,
	}
	assert.NoError(t, project_model.NewColumn(t.Context(), columnToDo))

	// Create label
	label := &issues_model.Label{
		RepoID: repo.ID,
		Name:   "bug",
		Color:  "ee0701",
	}
	assert.NoError(t, issues_model.NewLabel(t.Context(), label))

	session := loginUser(t, user.Name)

	// Create workflow via HTTP: when item is opened, move to "To Do" and add "bug" label
	testCreateProjectWorkflow(t, session, user.Name, repo.Name, project.ID, "item_opened", map[string]any{
		"event_id": string(project_model.WorkflowEventItemOpened),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnToDo.ID, 10),
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(label.ID, 10)},
		},
	})

	issueID := testNewIssueReturnIssue(t, session, repo, newIssueOptions{
		Title:     "Test Issue for Workflow",
		Content:   "This should trigger item_opened workflow",
		ProjectID: project.ID,
	})

	// Verify workflow executed: issue moved to "To Do" and has "bug" label
	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)

	projectIssue := &project_model.ProjectIssue{}
	has, err := db.GetEngine(t.Context()).Where("issue_id=?", issue.ID).Get(projectIssue)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, columnToDo.ID, projectIssue.ProjectColumnID)

	err = issue.LoadLabels(t.Context())
	assert.NoError(t, err)
	assert.Len(t, issue.Labels, 1)
	assert.Equal(t, label.ID, issue.Labels[0].ID)
}

func TestProjectWorkflowExecutionItemAddedToProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create project and columns
	project := &project_model.Project{
		Title:        "Test Workflow Execution",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	assert.NoError(t, project_model.NewProject(t.Context(), project))

	columnToDo := &project_model.Column{
		Title:     "To Do",
		ProjectID: project.ID,
	}
	assert.NoError(t, project_model.NewColumn(t.Context(), columnToDo))

	// Create label
	label := &issues_model.Label{
		RepoID: repo.ID,
		Name:   "bug",
		Color:  "ee0701",
	}
	assert.NoError(t, issues_model.NewLabel(t.Context(), label))

	session := loginUser(t, user.Name)

	// Create workflow via HTTP: when item added to project, move to "To Do" and add "bug" label
	testCreateProjectWorkflow(t, session, user.Name, repo.Name, project.ID, "item_added_to_project", map[string]any{
		"event_id": string(project_model.WorkflowEventItemAddedToProject),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnToDo.ID, 10),
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(label.ID, 10)},
		},
	})

	issueID := testNewIssueReturnIssue(t, session, repo, newIssueOptions{
		Title:   "Test Issue for Workflow",
		Content: "This should trigger workflow when added to project",
	})

	// Add issue to project via Web form - this triggers the workflow
	testAddIssueToProject(t, session, user.Name, repo.Name, project.ID, issueID)

	// Verify workflow executed: issue moved to "To Do" and has "bug" label
	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)

	projectIssue := &project_model.ProjectIssue{}
	has, err := db.GetEngine(t.Context()).Where("issue_id=?", issue.ID).Get(projectIssue)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, columnToDo.ID, projectIssue.ProjectColumnID)

	err = issue.LoadLabels(t.Context())
	assert.NoError(t, err)
	assert.Len(t, issue.Labels, 1)
	assert.Equal(t, label.ID, issue.Labels[0].ID)
}

func TestProjectWorkflowExecutionItemRemovedFromProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create project and columns
	project := &project_model.Project{
		Title:        "Test Workflow Execution",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	assert.NoError(t, project_model.NewProject(t.Context(), project))

	columnToDo := &project_model.Column{
		Title:     "To Do",
		ProjectID: project.ID,
	}
	assert.NoError(t, project_model.NewColumn(t.Context(), columnToDo))

	// Create label
	label := &issues_model.Label{
		RepoID: repo.ID,
		Name:   "no-project",
		Color:  "ee0701",
	}
	assert.NoError(t, issues_model.NewLabel(t.Context(), label))

	session := loginUser(t, user.Name)

	// Create workflow via HTTP: when item added to project, move to "To Do" and add "bug" label
	testCreateProjectWorkflow(t, session, user.Name, repo.Name, project.ID, "item_removed_from_project", map[string]any{
		"event_id": string(project_model.WorkflowEventItemRemovedFromProject),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(label.ID, 10)},
		},
	})

	issueID := testNewIssueReturnIssue(t, session, repo, newIssueOptions{
		Title:     "Test Issue for Workflow",
		Content:   "This should trigger workflow when removed from project",
		ProjectID: project.ID,
	})

	// remove issue from the project to trigger the workflow
	testAddIssueToProject(t, session, user.Name, repo.Name, 0, issueID)

	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.NoError(t, issue.LoadProject(t.Context()))
	assert.Nil(t, issue.Project)

	err = issue.LoadLabels(t.Context())
	assert.NoError(t, err)
	assert.Len(t, issue.Labels, 1)
	assert.Equal(t, label.ID, issue.Labels[0].ID)
}

// TestProjectWorkflowExecutionItemClosed tests workflow when issue is closed
func TestProjectWorkflowExecutionItemClosed(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	project := &project_model.Project{
		Title:        "Test Close Workflow",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	columnDone := &project_model.Column{
		Title:     "Done",
		ProjectID: project.ID,
	}
	err = project_model.NewColumn(t.Context(), columnDone)
	assert.NoError(t, err)

	labelCompleted := &issues_model.Label{
		RepoID: repo.ID,
		Name:   "completed",
		Color:  "00ff00",
	}
	err = issues_model.NewLabel(t.Context(), labelCompleted)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Create workflow: when closed, move to "Done" and add "completed" label
	testCreateProjectWorkflow(t, session, user.Name, repo.Name, project.ID, "item_closed", map[string]any{
		"event_id": string(project_model.WorkflowEventItemClosed),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnDone.ID, 10),
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(labelCompleted.ID, 10)},
		},
	})

	issueID := testNewIssueReturnIssue(t, session, repo, newIssueOptions{
		Title:     "Test Issue for Workflow",
		Content:   "This should trigger workflow when item is closed",
		ProjectID: project.ID,
	})

	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.False(t, issue.IsClosed)
	assert.NoError(t, issue.LoadRepo(t.Context()))

	// Close issue via API
	testIssueAddComment(t, session, issue.Link(), "Test comment 3", "close")

	// Verify workflow executed
	issue, err = issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.True(t, issue.IsClosed)

	projectIssue := &project_model.ProjectIssue{}
	has, err := db.GetEngine(t.Context()).Where("issue_id=?", issue.ID).Get(projectIssue)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, columnDone.ID, projectIssue.ProjectColumnID)

	err = issue.LoadLabels(t.Context())
	assert.NoError(t, err)
	hasLabel := false
	for _, l := range issue.Labels {
		if l.ID == labelCompleted.ID {
			hasLabel = true
			break
		}
	}
	assert.True(t, hasLabel)
}

func TestProjectWorkflowExecutionItemReopened(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	project := &project_model.Project{
		Title:        "Test Close Workflow",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	columnDone := &project_model.Column{
		Title:     "Done",
		ProjectID: project.ID,
	}
	err = project_model.NewColumn(t.Context(), columnDone)
	assert.NoError(t, err)

	labelCompleted := &issues_model.Label{
		RepoID: repo.ID,
		Name:   "completed",
		Color:  "00ff00",
	}
	err = issues_model.NewLabel(t.Context(), labelCompleted)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	testCreateProjectWorkflow(t, session, user.Name, repo.Name, project.ID, "item_reopened",
		map[string]any{
			"event_id": string(project_model.WorkflowEventItemReopened),
			"filters": map[string]any{
				string(project_model.WorkflowFilterTypeIssueType): "issue",
				string(project_model.WorkflowFilterTypeLabels):    strconv.FormatInt(labelCompleted.ID, 10),
			},
			"actions": map[string]any{
				string(project_model.WorkflowActionTypeColumn): strconv.FormatInt(columnDone.ID, 10),
			},
		})

	issueID := testNewIssueReturnIssue(t, session, repo, newIssueOptions{
		Title:     "Test Issue for Workflow",
		Content:   "This should trigger workflow when item is reopened",
		ProjectID: project.ID,
		LabelIDs:  []int64{labelCompleted.ID},
	})

	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.False(t, issue.IsClosed)
	assert.NoError(t, issue.LoadRepo(t.Context()))

	// Reopen issue
	testIssueAddComment(t, session, issue.Link(), "Test comment 3", "close")
	testIssueAddComment(t, session, issue.Link(), "Test comment 3", "reopen")

	// Reload and Verify workflow executed
	issue, err = issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.False(t, issue.IsClosed)

	projectIssue := &project_model.ProjectIssue{}
	has, err := db.GetEngine(t.Context()).Where("issue_id=?", issue.ID).Get(projectIssue)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, columnDone.ID, projectIssue.ProjectColumnID)

	err = issue.LoadLabels(t.Context())
	assert.NoError(t, err)
	hasLabel := false
	for _, l := range issue.Labels {
		if l.ID == labelCompleted.ID {
			hasLabel = true
			break
		}
	}
	assert.True(t, hasLabel)
}

// TestProjectWorkflowExecutionColumnChanged tests workflow when moving between columns
func TestProjectWorkflowExecutionColumnChanged(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	project := &project_model.Project{
		Title:        "Test Column Change",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	columnToDo := &project_model.Column{Title: "To Do", ProjectID: project.ID}
	err = project_model.NewColumn(t.Context(), columnToDo)
	assert.NoError(t, err)

	columnDone := &project_model.Column{Title: "Done", ProjectID: project.ID}
	err = project_model.NewColumn(t.Context(), columnDone)
	assert.NoError(t, err)

	labelWIP := &issues_model.Label{RepoID: repo.ID, Name: "wip", Color: "fbca04"}
	err = issues_model.NewLabel(t.Context(), labelWIP)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Create workflow: when moved to "Done", remove "wip" and close
	testCreateProjectWorkflow(t, session, user.Name, repo.Name, project.ID, "item_column_changed", map[string]any{
		"event_id": string(project_model.WorkflowEventItemColumnChanged),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeTargetColumn): strconv.FormatInt(columnDone.ID, 10),
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeRemoveLabels): []string{strconv.FormatInt(labelWIP.ID, 10)},
			string(project_model.WorkflowActionTypeIssueState):   "close",
		},
	})

	// Create issue with "wip" label
	issueID := testNewIssueReturnIssue(t, session, repo, newIssueOptions{
		Title:     "Test Column Change",
		Content:   "Will move columns",
		ProjectID: project.ID,
		LabelIDs:  []int64{labelWIP.ID},
	})

	// Move to "To Do" first
	moveReq := NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/%d/move?_csrf=%s", user.Name, repo.Name, project.ID, columnToDo.ID, GetUserCSRFToken(t, session)),
		map[string]any{
			"issues": []map[string]any{
				{
					"issueID": issueID,
					"sorting": 0,
				},
			},
		})
	session.MakeRequest(t, moveReq, http.StatusOK)

	// Move to "Done" - triggers workflow
	moveReq = NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/%d/move?_csrf=%s", user.Name, repo.Name, project.ID, columnDone.ID, GetUserCSRFToken(t, session)),
		map[string]any{
			"issues": []map[string]any{
				{
					"issueID": issueID,
					"sorting": 0,
				},
			},
		})
	session.MakeRequest(t, moveReq, http.StatusOK)

	// Verify workflow executed
	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.True(t, issue.IsClosed, "Issue should be closed")

	err = issue.LoadLabels(t.Context())
	assert.NoError(t, err)
	hasWIP := false
	for _, l := range issue.Labels {
		if l.ID == labelWIP.ID {
			hasWIP = true
			break
		}
	}
	assert.False(t, hasWIP, "WIP label should be removed")
}

func TestProjectWorkflowExecutionCodeChangesRequested(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Use existing PR #3 from fixtures (issue_id: 3, pull_request id: 2)
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))

	repo := pr.BaseRepo
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	project := &project_model.Project{
		Title:        "Test Code Changes Requested",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	columnInProgress := &project_model.Column{Title: "In Progress", ProjectID: project.ID}
	err = project_model.NewColumn(t.Context(), columnInProgress)
	assert.NoError(t, err)

	labelNeedChange := &issues_model.Label{RepoID: repo.ID, Name: "needs-changes", Color: "fbca04"}
	err = issues_model.NewLabel(t.Context(), labelNeedChange)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Create workflow: when code changes requested, add "needs-changes" label
	testCreateProjectWorkflow(t, session, user.Name, repo.Name, project.ID, "code_changes_requested", map[string]any{
		"event_id": string(project_model.WorkflowEventCodeChangesRequested),
		"filters":  map[string]any{},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(labelNeedChange.ID, 10)},
		},
	})

	// Add PR to project
	testAddIssueToProject(t, session, user.Name, repo.Name, project.ID, pr.Issue.ID)

	// User 2 submits a "REQUEST_CHANGES" review
	user2Session := loginUser(t, "user2")
	prURL := fmt.Sprintf("/%s/%s/pulls/%d", user.Name, repo.Name, pr.Issue.Index)
	req := NewRequest(t, "GET", prURL+"/files")
	resp := user2Session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	gitRepo, err := gitrepo.OpenRepository(t.Context(), pr.BaseRepo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	commitID, err := gitRepo.GetRefCommitID(pr.GetGitHeadRefName())
	assert.NoError(t, err)

	testSubmitReview(t, user2Session, htmlDoc.GetCSRF(), user.Name, repo.Name, strconv.FormatInt(pr.Issue.Index, 10), commitID, "reject", http.StatusOK)

	// Verify workflow executed: PR should have "needs-changes" label
	issue, err := issues_model.GetIssueByID(t.Context(), pr.Issue.ID)
	assert.NoError(t, err)
	err = issue.LoadLabels(t.Context())
	assert.NoError(t, err)

	hasNeedChangeLabel := false
	for _, l := range issue.Labels {
		if l.ID == labelNeedChange.ID {
			hasNeedChangeLabel = true
			break
		}
	}
	assert.True(t, hasNeedChangeLabel, "needs-changes label should be added")
}

func TestProjectWorkflowExecutionCodeReviewApproved(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Use existing PR #3 from fixtures (issue_id: 3, pull_request id: 2)
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))

	repo := pr.BaseRepo
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	project := &project_model.Project{
		Title:        "Test Code Review Approved",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	columnReadyToMerge := &project_model.Column{Title: "Ready to Merge", ProjectID: project.ID}
	err = project_model.NewColumn(t.Context(), columnReadyToMerge)
	assert.NoError(t, err)

	labelApproved := &issues_model.Label{RepoID: repo.ID, Name: "approved", Color: "00ff00"}
	err = issues_model.NewLabel(t.Context(), labelApproved)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Create workflow: when code review approved, move to "Ready to Merge" and add "approved" label
	testCreateProjectWorkflow(t, session, user.Name, repo.Name, project.ID, "code_review_approved", map[string]any{
		"event_id": string(project_model.WorkflowEventCodeReviewApproved),
		"filters":  map[string]any{},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnReadyToMerge.ID, 10),
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(labelApproved.ID, 10)},
		},
	})

	// Add PR to project
	testAddIssueToProject(t, session, user.Name, repo.Name, project.ID, pr.Issue.ID)

	// User 2 submits an "APPROVE" review
	user2Session := loginUser(t, "user2")
	prURL := fmt.Sprintf("/%s/%s/pulls/%d", user.Name, repo.Name, pr.Issue.Index)
	req := NewRequest(t, "GET", prURL+"/files")
	resp := user2Session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	gitRepo, err := gitrepo.OpenRepository(t.Context(), pr.BaseRepo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	commitID, err := gitRepo.GetRefCommitID(pr.GetGitHeadRefName())
	assert.NoError(t, err)

	testSubmitReview(t, user2Session, htmlDoc.GetCSRF(), user.Name, repo.Name, strconv.FormatInt(pr.Issue.Index, 10), commitID, "approve", http.StatusOK)

	// Verify workflow executed: PR should be in "Ready to Merge" column and have "approved" label
	issue, err := issues_model.GetIssueByID(t.Context(), pr.Issue.ID)
	assert.NoError(t, err)
	err = issue.LoadLabels(t.Context())
	assert.NoError(t, err)

	hasApprovedLabel := false
	for _, l := range issue.Labels {
		if l.ID == labelApproved.ID {
			hasApprovedLabel = true
			break
		}
	}
	assert.True(t, hasApprovedLabel, "approved label should be added")

	// Check column
	projectIssue := &project_model.ProjectIssue{}
	has, err := db.GetEngine(t.Context()).Where("issue_id=?", issue.ID).Get(projectIssue)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, columnReadyToMerge.ID, projectIssue.ProjectColumnID)
}

func TestProjectWorkflowExecutionPullRequestMerged(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// Fork repo1 and create a PR that can be merged
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited for merge test)\n")

		// Get the base repo
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})

		// Create project in base repo
		project := &project_model.Project{
			Title:        "Test PR Merged",
			RepoID:       repo.ID,
			Type:         project_model.TypeRepository,
			TemplateType: project_model.TemplateTypeNone,
		}
		err := project_model.NewProject(t.Context(), project)
		assert.NoError(t, err)

		columnDone := &project_model.Column{Title: "Done", ProjectID: project.ID}
		err = project_model.NewColumn(t.Context(), columnDone)
		assert.NoError(t, err)

		labelMerged := &issues_model.Label{RepoID: repo.ID, Name: "merged", Color: "6f42c1"}
		err = issues_model.NewLabel(t.Context(), labelMerged)
		assert.NoError(t, err)

		// Login as user2 (repo owner) to create workflow
		user2Session := loginUser(t, "user2")

		// Create workflow: when PR merged, move to "Done" and add "merged" label
		testCreateProjectWorkflow(t, user2Session, "user2", "repo1", project.ID, "pull_request_merged", map[string]any{
			"event_id": string(project_model.WorkflowEventPullRequestMerged),
			"filters":  map[string]any{},
			"actions": map[string]any{
				string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnDone.ID, 10),
				string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(labelMerged.ID, 10)},
			},
		})

		// Create PR from user1's fork to user2's repo
		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "Test PR for Merge Workflow")

		// Get PR details from redirect URL
		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])
		prNum := elem[4]

		// Load the PR
		prNumInt, err := strconv.ParseInt(prNum, 10, 64)
		assert.NoError(t, err)
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, Index: prNumInt})
		assert.NoError(t, pr.LoadIssue(t.Context()))

		// Add PR to project (as user2, the repo owner)
		testAddIssueToProject(t, user2Session, "user2", "repo1", project.ID, pr.Issue.ID)

		// Merge the PR (as user2, who has permission)
		prURL := "/user2/repo1/pulls/" + prNum
		req := NewRequest(t, "GET", prURL)
		resp = user2Session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		req = NewRequestWithValues(t, "POST", path.Join(prURL, "merge"), map[string]string{
			"_csrf": htmlDoc.GetCSRF(),
			"do":    string(repo_model.MergeStyleMerge),
		})
		user2Session.MakeRequest(t, req, http.StatusOK)

		// Verify workflow executed: PR should be in "Done" column and have "merged" label
		issue, err := issues_model.GetIssueByID(t.Context(), pr.Issue.ID)
		assert.NoError(t, err)
		err = issue.LoadLabels(t.Context())
		assert.NoError(t, err)

		hasMergedLabel := false
		for _, l := range issue.Labels {
			if l.ID == labelMerged.ID {
				hasMergedLabel = true
				break
			}
		}
		assert.True(t, hasMergedLabel, "merged label should be added")

		// Check column
		projectIssue := &project_model.ProjectIssue{}
		has, err := db.GetEngine(t.Context()).Where("issue_id=?", issue.ID).Get(projectIssue)
		assert.NoError(t, err)
		assert.True(t, has)
		assert.Equal(t, columnDone.ID, projectIssue.ProjectColumnID)

		// Verify PR is merged
		pr, err = issues_model.GetPullRequestByID(t.Context(), pr.ID)
		assert.NoError(t, err)
		assert.True(t, pr.HasMerged, "PR should be merged")
	})
}
