// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	activities_model "gitea.dev/models/activities"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToNotificationThreadIncludesRepoForAccessibleUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	n := newRepoNotification(t, 1, 4)
	thread := ToNotificationThread(t.Context(), n)

	if assert.NotNil(t, thread.Repository) {
		assert.Equal(t, n.Repository.FullName(), thread.Repository.FullName)
		assert.Nil(t, thread.Repository.Permissions)
	}
}

func TestToNotificationThreadOmitsRepoWhenAccessRevoked(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	n := newRepoNotification(t, 2, 4)
	thread := ToNotificationThread(t.Context(), n)

	assert.Nil(t, thread.Repository)
}

func TestToNotificationThreadOmitsSubjectWhenAccessRevoked(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	// repo 2 is private; user 4 has no access to it
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, repo.LoadOwner(ctx))
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4, RepoID: repo.ID})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	n := &activities_model.Notification{
		ID:          12345,
		UserID:      user.ID,
		RepoID:      repo.ID,
		Status:      activities_model.NotificationStatusUnread,
		Source:      activities_model.NotificationSourceIssue,
		IssueID:     issue.ID,
		UpdatedUnix: timeutil.TimeStampNow(),
		Issue:       issue,
		Repository:  repo,
		User:        user,
	}

	thread := ToNotificationThread(ctx, n)

	// must not leak private issue metadata once access is revoked
	assert.Nil(t, thread.Repository)
	assert.Nil(t, thread.Subject)
}

func TestToNotificationThread(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("issue notification", func(t *testing.T) {
		// Notification 1: source=issue, issue_id=1, status=unread
		n := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: 1})
		require.NoError(t, n.LoadAttributes(t.Context()))

		thread := ToNotificationThread(t.Context(), n)
		assert.Equal(t, int64(1), thread.ID)
		assert.True(t, thread.Unread)
		assert.False(t, thread.Pinned)
		require.NotNil(t, thread.Subject)
		assert.Equal(t, api.NotifySubjectIssue, thread.Subject.Type)
		assert.Equal(t, api.NotifySubjectStateOpen, thread.Subject.State)
	})

	t.Run("pinned notification", func(t *testing.T) {
		// Notification 3: status=pinned
		n := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: 3})
		require.NoError(t, n.LoadAttributes(t.Context()))

		thread := ToNotificationThread(t.Context(), n)
		assert.False(t, thread.Unread)
		assert.True(t, thread.Pinned)
	})

	t.Run("merged pull request returns merged state", func(t *testing.T) {
		// Issue 2 is a pull request; pull_request 1 has has_merged=true.
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})

		n := &activities_model.Notification{
			ID:         999,
			UserID:     2,
			RepoID:     repo.ID,
			Status:     activities_model.NotificationStatusUnread,
			Source:     activities_model.NotificationSourcePullRequest,
			IssueID:    issue.ID,
			Issue:      issue,
			Repository: repo,
		}

		thread := ToNotificationThread(t.Context(), n)
		require.NotNil(t, thread.Subject)
		assert.Equal(t, api.NotifySubjectPull, thread.Subject.Type)
		assert.Equal(t, api.NotifySubjectStateMerged, thread.Subject.State)
	})

	t.Run("open pull request returns open state", func(t *testing.T) {
		// Issue 3 is a pull request; pull_request 2 has has_merged=false.
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})

		n := &activities_model.Notification{
			ID:         998,
			UserID:     2,
			RepoID:     repo.ID,
			Status:     activities_model.NotificationStatusUnread,
			Source:     activities_model.NotificationSourcePullRequest,
			IssueID:    issue.ID,
			Issue:      issue,
			Repository: repo,
		}

		thread := ToNotificationThread(t.Context(), n)
		require.NotNil(t, thread.Subject)
		assert.Equal(t, api.NotifySubjectPull, thread.Subject.Type)
		assert.Equal(t, api.NotifySubjectStateOpen, thread.Subject.State)
	})
}

func newRepoNotification(t *testing.T, repoID, userID int64) *activities_model.Notification {
	t.Helper()

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	assert.NoError(t, repo.LoadOwner(ctx))
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})

	return &activities_model.Notification{
		ID:          repoID*1000 + userID,
		UserID:      user.ID,
		RepoID:      repo.ID,
		Status:      activities_model.NotificationStatusUnread,
		Source:      activities_model.NotificationSourceRepository,
		UpdatedUnix: timeutil.TimeStampNow(),
		Repository:  repo,
		User:        user,
	}
}
