// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"strings"
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/issues"

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

// TestIssueChangeStatusByWorkflowDoerNoFeedEntry verifies that when a project
// workflow (a virtual doer with ExtDoerData set) closes or reopens an issue,
// NO feed action is created. Without the guard the feed would show "Ghost"
// because the workflow doer's ID (-1) equals GhostUserID.
func TestIssueChangeStatusByWorkflowDoerNoFeedEntry(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.NoError(t, issue.LoadRepo(t.Context()))

	// Simulate the close comment that CloseIssue/ReopenIssue would create.
	closeComment := &issues_model.Comment{
		ID:      99999,
		Type:    issues_model.CommentTypeClose,
		IssueID: issue.ID,
	}

	workflowDoer := issues_model.NewProjectWorkflowDoer("My Project", 1, project_model.WorkflowEventItemClosed)

	// Count actions before the call.
	countBefore := unittest.GetCount(t, &activities_model.Action{})

	NewNotifier().IssueChangeStatus(t.Context(), workflowDoer, "", issue, closeComment, true)

	// No new action must have been recorded.
	countAfter := unittest.GetCount(t, &activities_model.Action{})
	assert.Equal(t, countBefore, countAfter, "workflow doer must not produce a feed entry")
}
