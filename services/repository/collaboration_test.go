// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepository_AddCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	for _, repo := range []*repo_model.Repository{repo1, repo1, repo3} {
		assert.NoError(t, repo.LoadOwner(t.Context()))
		assert.NoError(t, AddOrUpdateCollaborator(t.Context(), repo, user4, perm.AccessModeWrite))
		unittest.CheckConsistencyFor(t, repo, user4)
	}

	assert.Error(t, AddOrUpdateCollaborator(t.Context(), repo1, user4, perm.AccessModeOwner))
	assert.NoError(t, AddOrUpdateCollaborator(t.Context(), repo1, user4, perm.AccessModeAdmin))

	repo8 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 8})
	repo38 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 38})
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})
	for _, collab := range []struct {
		repo *repo_model.Repository
		user *user_model.User
	}{{repo8, user29}, {repo38, user4}} {
		assert.NoError(t, db.Insert(t.Context(), &repo_model.Collaboration{RepoID: collab.repo.ID, UserID: collab.user.ID, Mode: perm.AccessModeRead}))
		assert.NoError(t, AddOrUpdateCollaborator(t.Context(), collab.repo, collab.user, perm.AccessModeRead))
		unittest.AssertExistsAndLoadBean(t, &access_model.Access{RepoID: collab.repo.ID, UserID: collab.user.ID, Mode: perm.AccessModeRead})
	}
}

func TestRepository_DeleteCollaboration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 22})

	assert.NoError(t, repo.LoadOwner(t.Context()))
	for range 2 {
		assert.NoError(t, DeleteCollaboration(t.Context(), repo, user))
		unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: user.ID})
	}

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}

func TestRepository_DeleteCollaborationRemovesSubscriptionsAndStopwatches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 22})
	assert.NoError(t, repo.LoadOwner(ctx))
	assert.NoError(t, repo_model.WatchRepoAuto(ctx, user, repo, true))

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
	assert.False(t, repo_model.IsWatchModeWatching(watch.Mode))

	_, exists, err := issues_model.GetIssueWatch(ctx, user.ID, tempIssue.ID)
	assert.NoError(t, err)
	assert.False(t, exists)

	hasStopwatch, _, _, err := issues_model.HasUserStopwatch(ctx, user.ID)
	assert.NoError(t, err)
	assert.False(t, hasStopwatch)
}

func TestRepository_DeleteCollaborationPreservesWatchWithPublicAccess(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	assert.NoError(t, repo_model.UpdateRepoUnitPublicAccess(ctx, &repo_model.RepoUnit{
		RepoID: 2, Type: unit.TypeIssues, EveryoneAccessMode: perm.AccessModeRead,
	}))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, repo.LoadOwner(ctx))
	assert.NoError(t, AddOrUpdateCollaborator(ctx, repo, user, perm.AccessModeRead))
	assert.NoError(t, repo_model.WatchRepoAuto(ctx, user, repo, true))

	assert.NoError(t, DeleteCollaboration(ctx, repo, user))
	watch, err := repo_model.GetWatch(ctx, user.ID, repo.ID)
	assert.NoError(t, err)
	assert.True(t, repo_model.IsWatchModeWatching(watch.Mode))
}
