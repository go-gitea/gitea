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

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCreateProjectWorkflow(t *testing.T, session *TestSession, userName, repoName string, projectID int64, event string, workflowData map[string]any) {
	req := NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/workflows/%s", userName, repoName, projectID, event),
		workflowData)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var result map[string]any
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.True(t, result["success"].(bool))
}

// testNewIssueReturnIssue creates an issue through the web form and returns its ID.
//
// It resolves the issue from the redirect URL that testNewIssue already hands back
// rather than querying the repository for its newest issue. The children of
// TestProjectWorkflowExecutionIssues share one test environment, so a newest-issue
// lookup would be order-dependent and could latch onto a sibling's issue.
func testNewIssueReturnIssue(t *testing.T, session *TestSession, repo *repo_model.Repository, opts newIssueOptions) int64 {
	t.Helper()

	issueURL := testNewIssue(t, session, repo.OwnerName, repo.Name, opts)

	index, err := strconv.ParseInt(path.Base(issueURL), 10, 64)
	require.NoError(t, err, "unexpected issue URL %q", issueURL)

	issue, err := issues_model.GetIssueByIndex(t.Context(), repo.ID, index)
	require.NoError(t, err)
	return issue.ID
}

// testAddIssueToProject adds the issue to the project via web form if projectID == 0, it removes the issue from the project
func testAddIssueToProject(t *testing.T, session *TestSession, userName, repoName string, projectID, issueID int64) {
	projectValue := ""
	if projectID > 0 {
		projectValue = strconv.FormatInt(projectID, 10)
	}
	addToProjectReq := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/issues/projects?issue_ids=%d",
		userName, repoName, issueID),
		map[string]string{
			"id": projectValue,
		})
	session.MakeRequest(t, addToProjectReq, http.StatusOK)
}

// projectWorkflowExecFixture is the state shared by the children of the workflow
// execution parents. Only the owner/repo/session triple is hoisted: each child
// creates its own project, columns, labels and issue, so no mutable fixture row
// is shared between siblings.
type projectWorkflowExecFixture struct {
	user    *user_model.User
	repo    *repo_model.Repository
	session *TestSession
}

func (f *projectWorkflowExecFixture) newProject(t *testing.T, title string) *project_model.Project {
	t.Helper()
	project := &project_model.Project{
		Title:        title,
		RepoID:       f.repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	require.NoError(t, project_model.NewProject(t.Context(), project))
	return project
}

func (f *projectWorkflowExecFixture) newColumn(t *testing.T, project *project_model.Project, title string) *project_model.Column {
	t.Helper()
	column := &project_model.Column{Title: title, ProjectID: project.ID}
	require.NoError(t, project_model.NewColumn(t.Context(), column))
	return column
}

// newLabel creates a repository label. Callers must pass a name that is unique
// across the whole parent test: labels are created on the shared fixture repo and
// are not unique by name, so reusing a name would leave several identical labels
// on the repo and make the label pickers and filters ambiguous for later children.
func (f *projectWorkflowExecFixture) newLabel(t *testing.T, name, color string) *issues_model.Label {
	t.Helper()
	label := &issues_model.Label{RepoID: f.repo.ID, Name: name, Color: color}
	require.NoError(t, issues_model.NewLabel(t.Context(), label))
	return label
}

// assertIssueInColumn asserts that the issue's project card sits in the given column.
func assertIssueInColumn(t *testing.T, issueID, columnID int64) {
	t.Helper()
	projectIssue := &project_model.ProjectIssue{}
	has, err := db.GetEngine(t.Context()).Where("issue_id=?", issueID).Get(projectIssue)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, columnID, projectIssue.ProjectColumnID)
}

// assertIssueHasLabel asserts on the presence or absence of a label on an issue.
func assertIssueHasLabel(t *testing.T, issueID, labelID int64, expected bool, msgAndArgs ...any) {
	t.Helper()
	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.NoError(t, issue.LoadLabels(t.Context()))

	found := false
	for _, label := range issue.Labels {
		if label.ID == labelID {
			found = true
			break
		}
	}
	assert.Equal(t, expected, found, msgAndArgs...)
}

// TestProjectWorkflowExecutionIssues covers the workflow events that are driven by
// an issue's own lifecycle. Every child creates its own project, column, uniquely
// named label and issue, so the children are independent of each other's state.
func TestProjectWorkflowExecutionIssues(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	f := &projectWorkflowExecFixture{
		user: unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}),
		repo: unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}),
	}
	f.session = loginUser(t, f.user.Name)

	t.Run("ItemOpened", func(t *testing.T) { testProjectWorkflowExecutionItemOpened(t, f) })
	t.Run("ItemAddedToProject", func(t *testing.T) { testProjectWorkflowExecutionItemAddedToProject(t, f) })
	t.Run("ItemRemovedFromProject", func(t *testing.T) { testProjectWorkflowExecutionItemRemovedFromProject(t, f) })
	t.Run("ItemClosed", func(t *testing.T) { testProjectWorkflowExecutionItemClosed(t, f) })
	t.Run("ItemReopened", func(t *testing.T) { testProjectWorkflowExecutionItemReopened(t, f) })
	t.Run("ColumnChanged", func(t *testing.T) { testProjectWorkflowExecutionColumnChanged(t, f) })
}

// testProjectWorkflowExecutionItemOpened tests workflow execution when an issue is opened into a project
func testProjectWorkflowExecutionItemOpened(t *testing.T, f *projectWorkflowExecFixture) {
	project := f.newProject(t, "Test Workflow Execution ItemOpened")
	columnToDo := f.newColumn(t, project, "To Do")
	label := f.newLabel(t, "wf-opened-bug", "ee0701")

	// Create workflow via HTTP: when item is opened, move to "To Do" and add the label
	testCreateProjectWorkflow(t, f.session, f.user.Name, f.repo.Name, project.ID, "item_opened", map[string]any{
		"event_id": string(project_model.WorkflowEventItemOpened),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnToDo.ID, 10),
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(label.ID, 10)},
		},
	})

	issueID := testNewIssueReturnIssue(t, f.session, f.repo, newIssueOptions{
		Title:     "Test Issue for Workflow ItemOpened",
		Content:   "This should trigger item_opened workflow",
		ProjectID: project.ID,
	})

	// Verify workflow executed: issue moved to "To Do" and has the label
	assertIssueInColumn(t, issueID, columnToDo.ID)
	assertIssueHasLabel(t, issueID, label.ID, true)
}

func testProjectWorkflowExecutionItemAddedToProject(t *testing.T, f *projectWorkflowExecFixture) {
	project := f.newProject(t, "Test Workflow Execution ItemAddedToProject")
	columnToDo := f.newColumn(t, project, "To Do")
	label := f.newLabel(t, "wf-added-bug", "ee0701")

	// Create workflow via HTTP: when item added to project, move to "To Do" and add the label
	testCreateProjectWorkflow(t, f.session, f.user.Name, f.repo.Name, project.ID, "item_added_to_project", map[string]any{
		"event_id": string(project_model.WorkflowEventItemAddedToProject),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnToDo.ID, 10),
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(label.ID, 10)},
		},
	})

	issueID := testNewIssueReturnIssue(t, f.session, f.repo, newIssueOptions{
		Title:   "Test Issue for Workflow ItemAddedToProject",
		Content: "This should trigger workflow when added to project",
	})

	// Add issue to project via Web form - this triggers the workflow
	testAddIssueToProject(t, f.session, f.user.Name, f.repo.Name, project.ID, issueID)

	// Verify workflow executed: issue moved to "To Do" and has the label
	assertIssueInColumn(t, issueID, columnToDo.ID)
	assertIssueHasLabel(t, issueID, label.ID, true)
}

func testProjectWorkflowExecutionItemRemovedFromProject(t *testing.T, f *projectWorkflowExecFixture) {
	project := f.newProject(t, "Test Workflow Execution ItemRemovedFromProject")
	f.newColumn(t, project, "To Do")
	label := f.newLabel(t, "wf-removed-no-project", "ee0701")

	// Create workflow via HTTP: when item removed from project, add the label
	testCreateProjectWorkflow(t, f.session, f.user.Name, f.repo.Name, project.ID, "item_removed_from_project", map[string]any{
		"event_id": string(project_model.WorkflowEventItemRemovedFromProject),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(label.ID, 10)},
		},
	})

	issueID := testNewIssueReturnIssue(t, f.session, f.repo, newIssueOptions{
		Title:     "Test Issue for Workflow ItemRemovedFromProject",
		Content:   "This should trigger workflow when removed from project",
		ProjectID: project.ID,
	})

	// remove issue from the project to trigger the workflow
	testAddIssueToProject(t, f.session, f.user.Name, f.repo.Name, 0, issueID)

	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.NoError(t, issue.LoadProjects(t.Context()))
	assert.Empty(t, issue.Projects)

	assertIssueHasLabel(t, issueID, label.ID, true)
}

// testProjectWorkflowExecutionItemClosed tests workflow when issue is closed
func testProjectWorkflowExecutionItemClosed(t *testing.T, f *projectWorkflowExecFixture) {
	project := f.newProject(t, "Test Close Workflow")
	columnDone := f.newColumn(t, project, "Done")
	labelCompleted := f.newLabel(t, "wf-closed-completed", "00ff00")

	// Create workflow: when closed, move to "Done" and add the label
	testCreateProjectWorkflow(t, f.session, f.user.Name, f.repo.Name, project.ID, "item_closed", map[string]any{
		"event_id": string(project_model.WorkflowEventItemClosed),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnDone.ID, 10),
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(labelCompleted.ID, 10)},
		},
	})

	issueID := testNewIssueReturnIssue(t, f.session, f.repo, newIssueOptions{
		Title:     "Test Issue for Workflow ItemClosed",
		Content:   "This should trigger workflow when item is closed",
		ProjectID: project.ID,
	})

	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.False(t, issue.IsClosed)
	assert.NoError(t, issue.LoadRepo(t.Context()))

	// Close issue
	testIssueAddComment(t, f.session, issue.Link(), "Closing comment", "close")

	// Verify workflow executed
	issue, err = issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.True(t, issue.IsClosed)

	assertIssueInColumn(t, issueID, columnDone.ID)
	assertIssueHasLabel(t, issueID, labelCompleted.ID, true)
}

func testProjectWorkflowExecutionItemReopened(t *testing.T, f *projectWorkflowExecFixture) {
	project := f.newProject(t, "Test Reopen Workflow")
	columnDone := f.newColumn(t, project, "Done")
	labelCompleted := f.newLabel(t, "wf-reopened-completed", "00ff00")

	testCreateProjectWorkflow(t, f.session, f.user.Name, f.repo.Name, project.ID, "item_reopened",
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

	issueID := testNewIssueReturnIssue(t, f.session, f.repo, newIssueOptions{
		Title:     "Test Issue for Workflow ItemReopened",
		Content:   "This should trigger workflow when item is reopened",
		ProjectID: project.ID,
		LabelIDs:  []int64{labelCompleted.ID},
	})

	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.False(t, issue.IsClosed)
	assert.NoError(t, issue.LoadRepo(t.Context()))

	// Reopen issue
	testIssueAddComment(t, f.session, issue.Link(), "Closing comment", "close")
	testIssueAddComment(t, f.session, issue.Link(), "Reopening comment", "reopen")

	// Reload and Verify workflow executed
	issue, err = issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.False(t, issue.IsClosed)

	assertIssueInColumn(t, issueID, columnDone.ID)
	assertIssueHasLabel(t, issueID, labelCompleted.ID, true)
}

// testProjectWorkflowExecutionColumnChanged tests workflow when moving between columns
func testProjectWorkflowExecutionColumnChanged(t *testing.T, f *projectWorkflowExecFixture) {
	project := f.newProject(t, "Test Column Change")
	columnToDo := f.newColumn(t, project, "To Do")
	columnDone := f.newColumn(t, project, "Done")
	labelWIP := f.newLabel(t, "wf-column-wip", "fbca04")

	// Create workflow: when moved to "Done", remove the wip label and close
	testCreateProjectWorkflow(t, f.session, f.user.Name, f.repo.Name, project.ID, "item_column_changed", map[string]any{
		"event_id": string(project_model.WorkflowEventItemColumnChanged),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeTargetColumn): strconv.FormatInt(columnDone.ID, 10),
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeRemoveLabels): []string{strconv.FormatInt(labelWIP.ID, 10)},
			string(project_model.WorkflowActionTypeIssueState):   "close",
		},
	})

	// Create issue with the wip label
	issueID := testNewIssueReturnIssue(t, f.session, f.repo, newIssueOptions{
		Title:     "Test Column Change",
		Content:   "Will move columns",
		ProjectID: project.ID,
		LabelIDs:  []int64{labelWIP.ID},
	})

	moveIssueToColumn := func(columnID int64) {
		moveReq := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/%s/%s/projects/%d/%d/move", f.user.Name, f.repo.Name, project.ID, columnID),
			map[string]any{
				"issues": []map[string]any{
					{
						"issueID": issueID,
						"sorting": 0,
					},
				},
			})
		f.session.MakeRequest(t, moveReq, http.StatusOK)
	}

	// Move to "To Do" first, then to "Done" - the second move triggers the workflow
	moveIssueToColumn(columnToDo.ID)
	moveIssueToColumn(columnDone.ID)

	// Verify workflow executed
	issue, err := issues_model.GetIssueByID(t.Context(), issueID)
	assert.NoError(t, err)
	assert.True(t, issue.IsClosed, "Issue should be closed")

	assertIssueHasLabel(t, issueID, labelWIP.ID, false, "WIP label should be removed")
}

// TestProjectWorkflowExecutionReviews covers the workflow events driven by pull
// request reviews.
//
// Both children need an open pull request from the fixtures, and they deliberately
// use different ones: they add their PR to their own project and leave a review and
// a label on it, so sharing a single PR would let the first child's project
// membership, review state and labels leak into the second child's assertions.
func TestProjectWorkflowExecutionReviews(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	f := &projectWorkflowExecFixture{
		user: unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}),
		repo: unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}),
	}
	f.session = loginUser(t, f.user.Name)

	// pull_request fixture 2 (issue 3) and 5 (issue 11) are both open PRs on repo 1.
	t.Run("CodeChangesRequested", func(t *testing.T) { testProjectWorkflowExecutionCodeChangesRequested(t, f, 2) })
	t.Run("CodeReviewApproved", func(t *testing.T) { testProjectWorkflowExecutionCodeReviewApproved(t, f, 5) })
}

// loadWorkflowTestPullRequest loads a fixture pull request together with its issue
// and base repo, and asserts it belongs to the fixture's repository.
func (f *projectWorkflowExecFixture) loadPullRequest(t *testing.T, prID int64) *issues_model.PullRequest {
	t.Helper()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: prID})
	require.NoError(t, pr.LoadIssue(t.Context()))
	require.NoError(t, pr.LoadBaseRepo(t.Context()))
	require.Equal(t, f.repo.ID, pr.BaseRepoID)
	return pr
}

// submitReviewOnPullRequest submits a review of the given type on the pull request
// as the fixture user, resolving the head commit the review has to point at.
func (f *projectWorkflowExecFixture) submitReview(t *testing.T, pr *issues_model.PullRequest, reviewType string) {
	t.Helper()

	prURL := fmt.Sprintf("/%s/%s/pulls/%d", f.user.Name, f.repo.Name, pr.Issue.Index)
	req := NewRequest(t, "GET", prURL+"/files")
	f.session.MakeRequest(t, req, http.StatusOK)

	gitRepo, err := git.OpenRepository(pr.BaseRepo)
	require.NoError(t, err)
	defer gitRepo.Close()

	commitID, err := gitRepo.GetRefCommitID(t.Context(), pr.GetGitHeadRefName())
	require.NoError(t, err)

	testSubmitReview(t, f.session, f.user.Name, f.repo.Name, strconv.FormatInt(pr.Issue.Index, 10), commitID, reviewType, http.StatusOK)
}

func testProjectWorkflowExecutionCodeChangesRequested(t *testing.T, f *projectWorkflowExecFixture, prID int64) {
	pr := f.loadPullRequest(t, prID)

	project := f.newProject(t, "Test Code Changes Requested")
	f.newColumn(t, project, "In Progress")
	labelNeedChange := f.newLabel(t, "wf-review-needs-changes", "fbca04")

	// Create workflow: when code changes requested, add the needs-changes label
	testCreateProjectWorkflow(t, f.session, f.user.Name, f.repo.Name, project.ID, "code_changes_requested", map[string]any{
		"event_id": string(project_model.WorkflowEventCodeChangesRequested),
		"filters":  map[string]any{},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(labelNeedChange.ID, 10)},
		},
	})

	// Add PR to project
	testAddIssueToProject(t, f.session, f.user.Name, f.repo.Name, project.ID, pr.Issue.ID)

	f.submitReview(t, pr, "reject")

	// Verify workflow executed: PR should have the needs-changes label
	assertIssueHasLabel(t, pr.Issue.ID, labelNeedChange.ID, true, "needs-changes label should be added")
}

func testProjectWorkflowExecutionCodeReviewApproved(t *testing.T, f *projectWorkflowExecFixture, prID int64) {
	pr := f.loadPullRequest(t, prID)

	project := f.newProject(t, "Test Code Review Approved")
	columnReadyToMerge := f.newColumn(t, project, "Ready to Merge")
	labelApproved := f.newLabel(t, "wf-review-approved", "00ff00")

	// Create workflow: when code review approved, move to "Ready to Merge" and add the label
	testCreateProjectWorkflow(t, f.session, f.user.Name, f.repo.Name, project.ID, "code_review_approved", map[string]any{
		"event_id": string(project_model.WorkflowEventCodeReviewApproved),
		"filters":  map[string]any{},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn):    strconv.FormatInt(columnReadyToMerge.ID, 10),
			string(project_model.WorkflowActionTypeAddLabels): []string{strconv.FormatInt(labelApproved.ID, 10)},
		},
	})

	// Add PR to project
	testAddIssueToProject(t, f.session, f.user.Name, f.repo.Name, project.ID, pr.Issue.ID)

	f.submitReview(t, pr, "approve")

	// Verify workflow executed: PR should have the approved label and be in "Ready to Merge"
	assertIssueHasLabel(t, pr.Issue.ID, labelApproved.ID, true, "approved label should be added")
	assertIssueInColumn(t, pr.Issue.ID, columnReadyToMerge.ID)
}

// TestProjectWorkflowExecutionPullRequestMerged stays a standalone top-level test:
// it is the only workflow test that needs a live server (onGiteaRun) and it forks
// repo1 on disk, so it cannot share an environment with the other workflow tests.
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
		user2Session.MakeRequest(t, req, http.StatusOK)

		req = NewRequestWithValues(t, "POST", path.Join(prURL, "merge"), map[string]string{
			"do": string(repo_model.MergeStyleMerge),
		})
		user2Session.MakeRequest(t, req, http.StatusOK)

		// Verify workflow executed: PR should be in "Done" column and have "merged" label
		assertIssueHasLabel(t, pr.Issue.ID, labelMerged.ID, true, "merged label should be added")
		assertIssueInColumn(t, pr.Issue.ID, columnDone.ID)

		// Verify PR is merged
		pr, err = issues_model.GetPullRequestByID(t.Context(), pr.ID)
		assert.NoError(t, err)
		assert.True(t, pr.HasMerged, "PR should be merged")
	})
}
