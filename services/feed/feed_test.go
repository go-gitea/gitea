// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"testing"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	repository_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFeeds(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})

	for _, test := range []struct {
		requestedUser  *user_model.User
		actionID       int
		includeDeleted bool
	}{
		{user, 1, false}, // test with an individual user
		{org, 2, true},   // test with an organization user
	} {
		t.Run(test.requestedUser.Name, func(t *testing.T) {
			actions, count, err := GetFeeds(t.Context(), activities_model.GetFeedsOptions{
				RequestedUser:   test.requestedUser,
				Actor:           user,
				IncludePrivate:  true,
				OnlyPerformedBy: false,
				IncludeDeleted:  true,
			})
			assert.NoError(t, err)
			if assert.Len(t, actions, 1) {
				assert.EqualValues(t, test.actionID, actions[0].ID)
				assert.Equal(t, test.requestedUser.ID, actions[0].UserID)
			}
			assert.Equal(t, int64(1), count)

			actions, count, err = GetFeeds(t.Context(), activities_model.GetFeedsOptions{
				RequestedUser:   test.requestedUser,
				Actor:           user,
				IncludePrivate:  false,
				OnlyPerformedBy: false,
				IncludeDeleted:  test.includeDeleted,
			})
			assert.NoError(t, err)
			assert.Empty(t, actions)
			assert.Equal(t, int64(0), count)
		})
	}

	t.Run("removed collaborator cannot read their own feed's repository actions", func(t *testing.T) {
		collaborator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 22})
		action := &activities_model.Action{
			UserID: collaborator.ID, ActUserID: collaborator.ID, RepoID: repo.ID,
			OpType: activities_model.ActionRenameRepo, IsPrivate: true,
		}
		require.NoError(t, db.Insert(t.Context(), action))
		opts := activities_model.GetFeedsOptions{RequestedUser: collaborator, Actor: collaborator, IncludePrivate: true}
		actions, count, err := GetFeeds(t.Context(), opts)
		require.NoError(t, err)
		require.Len(t, actions, 1)
		assert.Equal(t, action.ID, actions[0].ID)
		assert.EqualValues(t, 1, count)

		require.NoError(t, repository_service.DeleteCollaboration(t.Context(), repo, collaborator))
		actions, count, err = GetFeeds(t.Context(), opts)
		require.NoError(t, err)
		assert.Empty(t, actions)
		assert.Zero(t, count)
		unittest.AssertExistsAndLoadBean(t, &activities_model.Action{ID: action.ID})
	})
}

func TestGetFeedsChecksUnitAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	// user4 reaches the private org3/repo3 only through team1, which is reduced to the Issues unit
	_, err := db.GetEngine(t.Context()).ID(2).Cols("authorize").Update(&organization.Team{AccessMode: perm.AccessModeNone})
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).Where("team_id = ? AND type <> ?", 2, unit.TypeIssues).Delete(&organization.TeamUnit{})
	require.NoError(t, err)
	for _, opType := range []activities_model.ActionType{
		activities_model.ActionCommitRepo,
		activities_model.ActionCreateIssue,
		activities_model.ActionCreatePullRequest,
		activities_model.ActionPublishRelease,
	} {
		for _, userID := range []int64{org3.ID, user4.ID} {
			require.NoError(t, db.Insert(t.Context(), &activities_model.Action{UserID: userID, ActUserID: 2, RepoID: 3, OpType: opType, IsPrivate: true}))
		}
	}

	feedOpTypes := func(requestedUser, actor *user_model.User) []activities_model.ActionType {
		actions, _, err := GetFeeds(t.Context(), activities_model.GetFeedsOptions{RequestedUser: requestedUser, Actor: actor, IncludePrivate: true})
		require.NoError(t, err)
		var opTypes []activities_model.ActionType
		for _, action := range actions {
			opTypes = append(opTypes, action.OpType)
		}
		return opTypes
	}
	assert.ElementsMatch(t, []activities_model.ActionType{activities_model.ActionRenameRepo, activities_model.ActionCreateIssue}, feedOpTypes(org3, user4))
	assert.ElementsMatch(t, []activities_model.ActionType{
		activities_model.ActionRenameRepo, activities_model.ActionCommitRepo, activities_model.ActionCreateIssue,
		activities_model.ActionCreatePullRequest, activities_model.ActionPublishRelease,
	}, feedOpTypes(org3, user2))
	_, err = db.GetEngine(t.Context()).ID(user2.ID).Cols("keep_activity_private").Update(&user_model.User{KeepActivityPrivate: true})
	require.NoError(t, err)
	assert.Equal(t, []activities_model.ActionType{activities_model.ActionCreateIssue}, feedOpTypes(user4, user4))
}

func TestGetFeedsForRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	privRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	pubRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 8})

	for _, test := range []struct {
		repo  *repo_model.Repository
		actor *user_model.User
		count int64
	}{
		{privRepo, nil, 0},  // private repo & no login
		{pubRepo, nil, 1},   // public repo & no login
		{privRepo, user, 1}, // private repo and login
		{pubRepo, user, 1},  // public repo & login
	} {
		actions, count, err := GetFeeds(t.Context(), activities_model.GetFeedsOptions{
			RequestedRepo:  test.repo,
			IncludePrivate: true,
			Actor:          test.actor,
		})
		assert.NoError(t, err)
		if test.count == 0 {
			assert.Empty(t, actions)
		} else {
			assert.Len(t, actions, 1)
		}
		assert.Equal(t, test.count, count)
	}
}

func TestGetFeedsCorrupted(t *testing.T) {
	// Now we will not check for corrupted data in the feeds
	// users should run doctor to fix their data
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ID:     8,
		RepoID: 1700,
	})

	actions, count, err := GetFeeds(t.Context(), activities_model.GetFeedsOptions{
		RequestedUser:  user,
		Actor:          user,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, int64(1), count)
}

func TestRepoActions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	_ = db.TruncateBeans(t.Context(), &activities_model.Action{})
	for i := range 3 {
		_ = db.Insert(t.Context(), &activities_model.Action{
			UserID:    2 + int64(i),
			ActUserID: 2,
			RepoID:    repo.ID,
			OpType:    activities_model.ActionCommentIssue,
		})
	}
	count, _ := db.Count[activities_model.Action](t.Context(), nil)
	assert.EqualValues(t, 3, count)
	actions, _, err := GetFeeds(t.Context(), activities_model.GetFeedsOptions{
		RequestedRepo: repo,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
}

func TestNotifyWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	action := &activities_model.Action{
		ActUserID: 8,
		RepoID:    1,
		OpType:    activities_model.ActionStarRepo,
	}
	assert.NoError(t, NotifyWatchers(t.Context(), action))

	// One watchers are inactive, thus action is only created for user 8, 1, 4, 11
	for _, userID := range []int64{8, 1, 4, 11} {
		unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
			ActUserID: action.ActUserID,
			UserID:    userID,
			RepoID:    action.RepoID,
			OpType:    action.OpType,
		})
	}
}

func TestNotifyWatchersRespectsWatchOptions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// user 1 watches repo 1 for issues only, user 4 keeps every event
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.NoError(t, repo_model.WatchRepoWithOptions(t.Context(), user1, repo1, repo_model.WatchOptions{Mode: repo_model.WatchModeNormal, WatchIssues: true}))

	assert.NoError(t, NotifyWatchers(t.Context(),
		&activities_model.Action{ActUserID: 8, RepoID: 1, OpType: activities_model.ActionCreateIssue},
		&activities_model.Action{ActUserID: 8, RepoID: 1, OpType: activities_model.ActionApprovePullRequest},
	))

	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{UserID: 1, RepoID: 1, OpType: activities_model.ActionCreateIssue})
	unittest.AssertNotExistsBean(t, &activities_model.Action{UserID: 1, RepoID: 1, OpType: activities_model.ActionApprovePullRequest})
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{UserID: 4, RepoID: 1, OpType: activities_model.ActionApprovePullRequest})
}
