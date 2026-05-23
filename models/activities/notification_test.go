// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"context"
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrUpdateIssueNotifications(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	assert.NoError(t, activities_model.CreateOrUpdateIssueNotifications(t.Context(), issue.ID, 0, 2, 0))

	// User 9 is inactive, thus notifications for user 1 and 4 are created
	notf := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 1, IssueID: issue.ID})
	assert.Equal(t, activities_model.NotificationStatusUnread, notf.Status)
	unittest.CheckConsistencyFor(t, &issues_model.Issue{ID: issue.ID})

	notf = unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 4, IssueID: issue.ID})
	assert.Equal(t, activities_model.NotificationStatusUnread, notf.Status)
}

func TestNotificationsForUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	notfs, err := db.Find[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: user.ID,
		Status: []activities_model.NotificationStatus{
			activities_model.NotificationStatusRead,
			activities_model.NotificationStatusUnread,
		},
	})
	assert.NoError(t, err)
	if assert.Len(t, notfs, 3) {
		assert.EqualValues(t, 5, notfs[0].ID)
		assert.Equal(t, user.ID, notfs[0].UserID)
		assert.EqualValues(t, 4, notfs[1].ID)
		assert.Equal(t, user.ID, notfs[1].UserID)
		assert.EqualValues(t, 2, notfs[2].ID)
		assert.Equal(t, user.ID, notfs[2].UserID)
	}
}

func TestNotification_GetRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	notf := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{RepoID: 1})
	repo, err := notf.GetRepo(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, repo, notf.Repository)
	assert.Equal(t, notf.RepoID, repo.ID)
}

func TestNotification_GetIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	notf := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{RepoID: 1})
	issue, err := notf.GetIssue(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, issue, notf.Issue)
	assert.Equal(t, notf.IssueID, issue.ID)
}

func TestGetNotificationCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	cnt, err := db.Count[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: user.ID,
		Status: []activities_model.NotificationStatus{
			activities_model.NotificationStatusRead,
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, cnt)

	cnt, err = db.Count[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: user.ID,
		Status: []activities_model.NotificationStatus{
			activities_model.NotificationStatusUnread,
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
}

func TestSetNotificationStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	notf := unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{UserID: user.ID, Status: activities_model.NotificationStatusRead})
	_, err := activities_model.SetNotificationStatus(t.Context(), notf.ID, user, activities_model.NotificationStatusPinned)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{ID: notf.ID, Status: activities_model.NotificationStatusPinned})

	_, err = activities_model.SetNotificationStatus(t.Context(), 1, user, activities_model.NotificationStatusRead)
	assert.Error(t, err)
	_, err = activities_model.SetNotificationStatus(t.Context(), unittest.NonexistentID, user, activities_model.NotificationStatusRead)
	assert.Error(t, err)
}

func TestUpdateNotificationStatuses(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	notfUnread := unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{UserID: user.ID, Status: activities_model.NotificationStatusUnread})
	notfRead := unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{UserID: user.ID, Status: activities_model.NotificationStatusRead})
	notfPinned := unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{UserID: user.ID, Status: activities_model.NotificationStatusPinned})
	assert.NoError(t, activities_model.UpdateNotificationStatuses(t.Context(), user, activities_model.NotificationStatusUnread, activities_model.NotificationStatusRead))
	unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{ID: notfUnread.ID, Status: activities_model.NotificationStatusRead})
	unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{ID: notfRead.ID, Status: activities_model.NotificationStatusRead})
	unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{ID: notfPinned.ID, Status: activities_model.NotificationStatusPinned})
}

func TestSetIssueReadBy(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		return activities_model.SetIssueReadBy(ctx, issue.ID, user.ID)
	}))

	nt, err := activities_model.GetIssueNotification(t.Context(), user.ID, issue.ID)
	assert.NoError(t, err)
	assert.Equal(t, activities_model.NotificationStatusRead, nt.Status)
}

func TestGetIssueNotificationUsesUniqueKeyForPullRequests(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2, IsPull: true})
	assert.NoError(t, activities_model.CreateOrUpdateIssueNotifications(t.Context(), issue.ID, 0, 1, 4))

	nt, err := activities_model.GetIssueNotification(t.Context(), 4, issue.ID)
	assert.NoError(t, err)
	assert.Equal(t, issue.ID, nt.IssueID)
	assert.Equal(t, activities_model.NotificationSourcePullRequest, nt.Source)
}

func TestGetIssueNotificationReturnsErrNotExistWhenMissing(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	opts := activities_model.FindNotificationOptions{UserID: 1}
	opts.FilterByIssue(issue.ID, issue.IsPull)
	_, err := db.GetEngine(t.Context()).Where(opts.ToConds()).Delete(&activities_model.Notification{})
	assert.NoError(t, err)

	_, err = activities_model.GetIssueNotification(t.Context(), 1, issue.ID)
	assert.Error(t, err)
	assert.True(t, db.IsErrNotExist(err))
}

func TestCreateCommitNotificationsDeduplicatesByRepoAndCommit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const commitID = "0123456789abcdef"
	const firstRepoID = int64(1)
	const secondRepoID = int64(2)

	assert.NoError(t, activities_model.CreateCommitNotifications(t.Context(), 1, firstRepoID, commitID, receiverID))
	assert.NoError(t, activities_model.CreateCommitNotifications(t.Context(), 3, firstRepoID, commitID, receiverID))
	assert.NoError(t, activities_model.CreateCommitNotifications(t.Context(), 4, secondRepoID, commitID, receiverID))

	notfs, err := db.Find[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: receiverID,
		Source: []activities_model.NotificationSource{activities_model.NotificationSourceCommit},
	})
	assert.NoError(t, err)
	if assert.Len(t, notfs, 2) {
		assert.Equal(t, commitID, notfs[0].CommitID)
		assert.Equal(t, commitID, notfs[1].CommitID)
		assert.ElementsMatch(t, []int64{firstRepoID, secondRepoID}, []int64{notfs[0].RepoID, notfs[1].RepoID})

		var firstRepoNotification *activities_model.Notification
		for _, notf := range notfs {
			if notf.RepoID == firstRepoID {
				firstRepoNotification = notf
				break
			}
		}
		if assert.NotNil(t, firstRepoNotification) {
			assert.Equal(t, activities_model.NotificationStatusUnread, firstRepoNotification.Status)
			assert.EqualValues(t, 3, firstRepoNotification.UpdatedBy)
		}
	}
}

func TestCreateOrUpdateReleaseNotificationsDeduplicatesByRelease(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const repoID = int64(1)
	const releaseID = int64(1)

	assert.NoError(t, activities_model.CreateOrUpdateReleaseNotifications(t.Context(), 1, repoID, releaseID, receiverID))
	assert.NoError(t, activities_model.CreateOrUpdateReleaseNotifications(t.Context(), 3, repoID, releaseID, receiverID))

	opts := activities_model.FindNotificationOptions{
		UserID: receiverID,
		Source: []activities_model.NotificationSource{activities_model.NotificationSourceRelease},
	}
	opts.FilterByRelease(releaseID)

	notfs, err := db.Find[activities_model.Notification](t.Context(), opts)
	assert.NoError(t, err)
	if assert.Len(t, notfs, 1) {
		assert.Equal(t, activities_model.NotificationStatusUnread, notfs[0].Status)
		assert.EqualValues(t, 3, notfs[0].UpdatedBy)
		assert.Equal(t, releaseID, notfs[0].ReleaseID)
	}
}

func TestSetCommitReadByScopesToRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const commitID = "fedcba9876543210"
	const firstRepoID = int64(1)
	const secondRepoID = int64(2)

	assert.NoError(t, activities_model.CreateCommitNotifications(t.Context(), 1, firstRepoID, commitID, receiverID))
	assert.NoError(t, activities_model.CreateCommitNotifications(t.Context(), 1, secondRepoID, commitID, receiverID))
	assert.NoError(t, activities_model.SetCommitReadBy(t.Context(), firstRepoID, receiverID, commitID))

	firstRepoNotification := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID:   receiverID,
		RepoID:   firstRepoID,
		Source:   activities_model.NotificationSourceCommit,
		CommitID: commitID,
	})
	secondRepoNotification := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID:   receiverID,
		RepoID:   secondRepoID,
		Source:   activities_model.NotificationSourceCommit,
		CommitID: commitID,
	})

	assert.Equal(t, activities_model.NotificationStatusRead, firstRepoNotification.Status)
	assert.Equal(t, activities_model.NotificationStatusUnread, secondRepoNotification.Status)
}

func TestFindNotificationOptionsCombinesUniqueKeyWithStatusAndSource(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const repoID = int64(1)
	const releaseID = int64(1)

	// Seed an unread release notification.
	assert.NoError(t, activities_model.CreateOrUpdateReleaseNotifications(t.Context(), 1, repoID, releaseID, receiverID))

	// Combining FilterByRelease with a Status filter that excludes the row must
	// return an empty result. The previous implementation silently dropped the
	// Status filter when a unique key was set, masking this kind of query bug.
	opts := activities_model.FindNotificationOptions{
		UserID: receiverID,
		Status: []activities_model.NotificationStatus{activities_model.NotificationStatusRead},
	}
	opts.FilterByRelease(releaseID)

	notfs, err := db.Find[activities_model.Notification](t.Context(), opts)
	assert.NoError(t, err)
	assert.Empty(t, notfs, "Status filter must be honoured when uniqueKey is set")

	// And combining with a Source filter that does not match must also return empty.
	opts = activities_model.FindNotificationOptions{
		UserID: receiverID,
		Source: []activities_model.NotificationSource{activities_model.NotificationSourceIssue},
	}
	opts.FilterByRelease(releaseID)
	notfs, err = db.Find[activities_model.Notification](t.Context(), opts)
	assert.NoError(t, err)
	assert.Empty(t, notfs, "Source filter must be honoured when uniqueKey is set")
}

func TestUpsertNotificationByUniqueKeyIsIdempotent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const repoID = int64(1)
	const releaseID = int64(1)

	// Repeated calls for the same release/user must converge to one row regardless
	// of how many times the upsert runs (covers the retry-on-conflict path).
	for range 5 {
		assert.NoError(t, activities_model.CreateOrUpdateReleaseNotifications(t.Context(), 1, repoID, releaseID, receiverID))
	}

	opts := activities_model.FindNotificationOptions{UserID: receiverID}
	opts.FilterByRelease(releaseID)
	notfs, err := db.Find[activities_model.Notification](t.Context(), opts)
	assert.NoError(t, err)
	assert.Len(t, notfs, 1)
}

func TestCreateRepoTransferNotificationDeduplicatesByRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const repoID = int64(1)

	assert.NoError(t, activities_model.CreateRepoTransferNotification(t.Context(), 1, repoID, receiverID))
	assert.NoError(t, activities_model.CreateRepoTransferNotification(t.Context(), 3, repoID, receiverID))

	notfs, err := db.Find[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: receiverID,
		RepoID: repoID,
		Source: []activities_model.NotificationSource{activities_model.NotificationSourceRepository},
	})
	assert.NoError(t, err)
	if assert.Len(t, notfs, 1) {
		assert.Equal(t, activities_model.NotificationStatusUnread, notfs[0].Status)
		assert.EqualValues(t, 3, notfs[0].UpdatedBy)
	}
}
