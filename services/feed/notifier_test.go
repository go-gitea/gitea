// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"strings"
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"

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

func TestPullRequestReviewRequestAction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}) // This is a pull request (is_pull: true)
	assert.NoError(t, issue.LoadRepo(t.Context()))
	issue.Repo.Owner = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})

	// Test requesting a review (isRequest = true)
	actionBean := &activities_model.Action{
		OpType:    activities_model.ActionPullRequestReviewRequest,
		ActUserID: doer.ID,
		RepoID:    issue.RepoID,
		IsPrivate: issue.Repo.IsPrivate,
	}
	unittest.AssertNotExistsBean(t, actionBean)

	NewNotifier().PullRequestReviewRequest(t.Context(), doer, issue, reviewer, true, nil)

	unittest.AssertExistsAndLoadBean(t, actionBean)
	unittest.CheckConsistencyFor(t, &activities_model.Action{})
}

func TestIssueChangeAssigneeAction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assignee := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}) // Regular issue (is_pull: false)
	assert.NoError(t, issue.LoadRepo(t.Context()))
	issue.Repo.Owner = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})

	// Test assigning (removed = false)
	actionBean := &activities_model.Action{
		OpType:    activities_model.ActionIssueChangeAssignee,
		ActUserID: doer.ID,
		RepoID:    issue.RepoID,
		IsPrivate: issue.Repo.IsPrivate,
	}
	unittest.AssertNotExistsBean(t, actionBean)

	NewNotifier().IssueChangeAssignee(t.Context(), doer, issue, assignee, false, nil)

	unittest.AssertExistsAndLoadBean(t, actionBean)
	unittest.CheckConsistencyFor(t, &activities_model.Action{})
}
