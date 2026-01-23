// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepository_AddCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(repoID, userID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		assert.NoError(t, repo.LoadOwner(t.Context()))
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
		assert.NoError(t, AddOrUpdateCollaborator(t.Context(), repo, user, perm.AccessModeWrite))
		unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repoID}, &user_model.User{ID: userID})
	}
	testSuccess(1, 4)
	testSuccess(1, 4)
	testSuccess(3, 4)
}

func TestRepository_DeleteCollaboration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 22})

	assert.NoError(t, repo.LoadOwner(t.Context()))
	assert.NoError(t, DeleteCollaboration(t.Context(), repo, user))
	unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: user.ID})

	assert.NoError(t, DeleteCollaboration(t.Context(), repo, user))
	unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: user.ID})

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}

func TestRepository_DeleteCollaborationRemovesSubscriptionsAndStopwatches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 22})
	assert.NoError(t, repo.LoadOwner(ctx))
	assert.NoError(t, repo_model.WatchRepo(ctx, user, repo, true))

	hasAccess, err := access_model.HasAnyUnitAccess(ctx, user.ID, repo)
	assert.NoError(t, err)
	assert.True(t, hasAccess)

	issueCount, err := db.GetEngine(ctx).Where("repo_id=?", repo.ID).Count(new(issues_model.Issue))
	assert.NoError(t, err)
	tempIssue := &issues_model.Issue{
		RepoID:   repo.ID,
		Index:    issueCount + 1,
		PosterID: repo.OwnerID,
		Title:    "temp issue",
		Content:  "temp",
	}
	assert.NoError(t, db.Insert(ctx, tempIssue))
	assert.NoError(t, issues_model.CreateOrUpdateIssueWatch(ctx, user.ID, tempIssue.ID, true))
	ok, err := issues_model.CreateIssueStopwatch(ctx, user, tempIssue)
	assert.NoError(t, err)
	assert.True(t, ok)

	assert.NoError(t, DeleteCollaboration(ctx, repo, user))

	hasAccess, err = access_model.HasAnyUnitAccess(ctx, user.ID, repo)
	assert.NoError(t, err)
	assert.False(t, hasAccess)

	watch, err := repo_model.GetWatch(ctx, user.ID, repo.ID)
	assert.NoError(t, err)
	assert.False(t, repo_model.IsWatchMode(watch.Mode))

	_, exists, err := issues_model.GetIssueWatch(ctx, user.ID, tempIssue.ID)
	assert.NoError(t, err)
	assert.False(t, exists)

	hasStopwatch, _, _, err := issues_model.HasUserStopwatch(ctx, user.ID)
	assert.NoError(t, err)
	assert.False(t, hasStopwatch)
}
