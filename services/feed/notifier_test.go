// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"fmt"
	"strings"
	"testing"

	activities_model "gitea.dev/models/activities"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestRenameRepoAction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID})
	repo.Owner = user

	oldRepoName := repo.Name
	const newRepoName = "newRepoName"
	repo.Name = newRepoName
	repo.LowerName = strings.ToLower(newRepoName)

	actionBean := &activities_model.Action{
		OpType:    activities_model.ActionRenameRepo,
		ActUserID: user.ID,
		ActUser:   user,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	}
	unittest.AssertNotExistsBean(t, actionBean)

	NewNotifier().RenameRepository(t.Context(), user, repo, oldRepoName)

	unittest.AssertExistsAndLoadBean(t, actionBean)
	unittest.CheckConsistencyFor(t, &activities_model.Action{})
}

// TestIssueChangeStatusByWorkflowDoerAttributesToTriggeringUser verifies that
// when a project workflow closes or reopens an issue, the feed action is
// attributed to the REAL user whose action triggered the workflow (see
// issues_model.NewProjectWorkflowDoer), and is recorded like any other
// IssueChangeStatus - exactly as an issue auto-closed by a commit message
// (services/issue/commit.go) is attributed to the pushing user, feed entry
// included. ExtDoerData/CommentMetaData is what records this was automated;
// it is not a reason to hide the action from the feed.
func TestIssueChangeStatusByWorkflowDoerAttributesToTriggeringUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	triggeringUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.NoError(t, issue.LoadRepo(t.Context()))

	// Simulate the close comment that CloseIssue/ReopenIssue would create.
	closeComment := &issues_model.Comment{
		ID:      99999,
		Type:    issues_model.CommentTypeClose,
		IssueID: issue.ID,
	}

	workflowDoer := issues_model.NewProjectWorkflowDoer(triggeringUser, "My Project", 1, project_model.WorkflowEventItemClosed)
	assert.True(t, issues_model.IsProjectWorkflowDoer(workflowDoer), "sanity check: doer must still be recognized as a workflow doer")
	assert.Equal(t, triggeringUser.ID, workflowDoer.ID, "the doer's identity must be the real triggering user")

	actionBean := &activities_model.Action{
		OpType:    activities_model.ActionCloseIssue,
		ActUserID: triggeringUser.ID,
		ActUser:   triggeringUser,
		Content:   fmt.Sprintf("%d|%s", issue.Index, ""),
		RepoID:    issue.Repo.ID,
		Repo:      issue.Repo,
		CommentID: closeComment.ID,
		IsPrivate: issue.Repo.IsPrivate,
	}
	unittest.AssertNotExistsBean(t, actionBean)

	NewNotifier().IssueChangeStatus(t.Context(), workflowDoer, "", issue, closeComment, true)

	unittest.AssertExistsAndLoadBean(t, actionBean)
}
