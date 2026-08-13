// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"context"
	"testing"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrUpdateIssueNotifications(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	_, err := activities_model.CreateOrUpdateIssueNotifications(t.Context(), issue.ID, 0, 2, 0)
	assert.NoError(t, err)

	// User 9 is inactive, thus notifications for user 1 and 4 are created
	notf := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID: 1, Source: activities_model.NotificationSourceIssue, SubjectID: issue.ID,
	})
	assert.Equal(t, activities_model.NotificationStatusUnread, notf.Status)
	unittest.CheckConsistencyFor(t, &issues_model.Issue{ID: issue.ID})

	notf = unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID: 4, Source: activities_model.NotificationSourceIssue, SubjectID: issue.ID,
	})
	assert.Equal(t, activities_model.NotificationStatusUnread, notf.Status)
}

// The title is snapshotted when the notification is written so the list never has to load
// the issue to render a row.
func TestCreateOrUpdateIssueNotificationsSnapshotsTitle(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	_, err := activities_model.CreateOrUpdateIssueNotifications(t.Context(), issue.ID, 0, 2, 0)
	require.NoError(t, err)

	notf := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID: 4, Source: activities_model.NotificationSourceIssue, SubjectID: issue.ID,
	})
	assert.Equal(t, issue.Title, notf.Title)
}

func TestCreateOrUpdateIssueNotificationsForAssigneeAndReviewer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// user 13 neither watches repo 1 nor participates in PR 3
	assert.NoError(t, db.Insert(t.Context(), &issues_model.IssueAssignees{AssigneeID: 13, IssueID: 3}))
	_, err := activities_model.CreateOrUpdateIssueNotifications(t.Context(), 3, 0, 1, 0)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID: 13, Source: activities_model.NotificationSourcePullRequest, SubjectID: 3,
	})

	// user 1 is a requested reviewer of PR 12 and does not participate in it
	_, err = activities_model.CreateOrUpdateIssueNotifications(t.Context(), 12, 0, 2, 0)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID: 1, Source: activities_model.NotificationSourcePullRequest, SubjectID: 12,
	})
}

func TestCreateOrUpdateIssueNotificationsIgnored(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// user 4 watches repo 1 and would be notified about issue 1
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	assert.NoError(t, repo_model.WatchIgnoreRepo(t.Context(), user, repo))

	notified, err := activities_model.CreateOrUpdateIssueNotifications(t.Context(), 1, 0, 2, 0)
	assert.NoError(t, err)
	assert.NotContains(t, notified, user.ID)

	// muting outranks a direct receiver too
	notified, err = activities_model.CreateOrUpdateIssueNotifications(t.Context(), 1, 0, 2, user.ID)
	assert.NoError(t, err)
	assert.Empty(t, notified)
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
	assert.Equal(t, notf.IssueID(), issue.ID)
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
	_, err := activities_model.UpdateNotificationStatuses(t.Context(), user, activities_model.NotificationStatusUnread, activities_model.NotificationStatusRead)
	assert.NoError(t, err)
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
		changed, err := activities_model.SetIssueReadBy(ctx, issue.ID, user.ID)
		assert.True(t, changed, "an unread notification must report that it changed")
		return err
	}))

	nt, err := activities_model.GetIssueNotification(t.Context(), user.ID, issue.ID)
	assert.NoError(t, err)
	assert.Equal(t, activities_model.NotificationStatusRead, nt.Status)
}

// Callers use the bool to decide whether to push an unread-count update, so a second read
// of the same notification must report no change.
func TestSetIssueReadByReportsNoChangeWhenAlreadyRead(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	changed, err := activities_model.SetIssueReadBy(t.Context(), issue.ID, user.ID)
	require.NoError(t, err)
	require.True(t, changed)

	changed, err = activities_model.SetIssueReadBy(t.Context(), issue.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, changed, "reading an already-read notification must not report a change")
}

// A user with no notification for the issue at all is a no-op, not an error.
func TestSetIssueReadByIsNoOpWhenNoNotificationExists(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	changed, err := activities_model.SetIssueReadBy(t.Context(), issue.ID, 4)
	assert.NoError(t, err)
	assert.False(t, changed)
}

func TestGetIssueNotificationMatchesPullRequestSource(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2, IsPull: true})
	_, err := activities_model.CreateOrUpdateIssueNotifications(t.Context(), issue.ID, 0, 1, 4)
	assert.NoError(t, err)

	nt, err := activities_model.GetIssueNotification(t.Context(), 4, issue.ID)
	assert.NoError(t, err)
	assert.Equal(t, issue.ID, nt.IssueID())
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

func TestCreateCommitNotificationDeduplicatesByRepoAndCommit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const commitID = "0123456789abcdef"
	const firstRepoID = int64(1)
	const secondRepoID = int64(2)

	_, err := activities_model.CreateCommitNotification(t.Context(), 1, firstRepoID, commitID, receiverID, "first title")
	require.NoError(t, err)
	_, err = activities_model.CreateCommitNotification(t.Context(), 3, firstRepoID, commitID, receiverID, "second title")
	require.NoError(t, err)
	_, err = activities_model.CreateCommitNotification(t.Context(), 4, secondRepoID, commitID, receiverID, "other repo")
	require.NoError(t, err)

	notfs, err := db.Find[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: receiverID,
		Source: []activities_model.NotificationSource{activities_model.NotificationSourceCommit},
	})
	assert.NoError(t, err)
	if assert.Len(t, notfs, 2, "the same sha in two repos stays two notifications") {
		assert.Equal(t, commitID, notfs[0].CommitID())
		assert.Equal(t, commitID, notfs[1].CommitID())
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

func TestCreateReleaseNotificationDeduplicatesByRelease(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const repoID = int64(1)
	const releaseID = int64(1)

	_, err := activities_model.CreateReleaseNotification(t.Context(), 1, repoID, releaseID, receiverID, "v1.0")
	require.NoError(t, err)
	_, err = activities_model.CreateReleaseNotification(t.Context(), 3, repoID, releaseID, receiverID, "v1.0")
	require.NoError(t, err)

	opts := activities_model.FindNotificationOptions{UserID: receiverID}
	opts.FilterByRelease(releaseID)

	notfs, err := db.Find[activities_model.Notification](t.Context(), opts)
	assert.NoError(t, err)
	if assert.Len(t, notfs, 1) {
		assert.Equal(t, activities_model.NotificationStatusUnread, notfs[0].Status)
		assert.EqualValues(t, 3, notfs[0].UpdatedBy)
		assert.Equal(t, releaseID, notfs[0].ReleaseID())
	}
}

func TestSetCommitReadByScopesToRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const commitID = "fedcba9876543210"
	const firstRepoID = int64(1)
	const secondRepoID = int64(2)

	_, err := activities_model.CreateCommitNotification(t.Context(), 1, firstRepoID, commitID, receiverID, "title")
	require.NoError(t, err)
	_, err = activities_model.CreateCommitNotification(t.Context(), 1, secondRepoID, commitID, receiverID, "title")
	require.NoError(t, err)

	changed, err := activities_model.SetCommitReadBy(t.Context(), firstRepoID, receiverID, commitID)
	require.NoError(t, err)
	assert.True(t, changed)

	firstRepoNotification := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID:     receiverID,
		RepoID:     firstRepoID,
		Source:     activities_model.NotificationSourceCommit,
		SubjectRef: commitID,
	})
	secondRepoNotification := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID:     receiverID,
		RepoID:     secondRepoID,
		Source:     activities_model.NotificationSourceCommit,
		SubjectRef: commitID,
	})

	assert.Equal(t, activities_model.NotificationStatusRead, firstRepoNotification.Status)
	assert.Equal(t, activities_model.NotificationStatusUnread, secondRepoNotification.Status)
}

func TestSetRepoReadByScopesToRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)

	_, err := activities_model.CreateRepoTransferNotification(t.Context(), 1, 1, receiverID, "user2/repo1")
	require.NoError(t, err)
	_, err = activities_model.CreateRepoTransferNotification(t.Context(), 1, 2, receiverID, "user2/repo2")
	require.NoError(t, err)

	changed, err := activities_model.SetRepoReadBy(t.Context(), receiverID, 1)
	require.NoError(t, err)
	assert.True(t, changed)

	first := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID: receiverID, Source: activities_model.NotificationSourceRepository, RepoID: 1,
	})
	second := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{
		UserID: receiverID, Source: activities_model.NotificationSourceRepository, RepoID: 2,
	})
	assert.Equal(t, activities_model.NotificationStatusRead, first.Status)
	assert.Equal(t, activities_model.NotificationStatusUnread, second.Status, "reading one repo must not read another")
}

// Filters must compose: the subject filter narrows the query, it does not replace the
// status filter. An earlier design silently dropped Status when a subject key was set.
func TestFindNotificationOptionsCombineSubjectAndStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const releaseID = int64(1)

	_, err := activities_model.CreateReleaseNotification(t.Context(), 1, 1, releaseID, receiverID, "v1.0")
	require.NoError(t, err)

	opts := activities_model.FindNotificationOptions{
		UserID: receiverID,
		Status: []activities_model.NotificationStatus{activities_model.NotificationStatusRead},
	}
	opts.FilterByRelease(releaseID)

	notfs, err := db.Find[activities_model.Notification](t.Context(), opts)
	assert.NoError(t, err)
	assert.Empty(t, notfs, "Status filter must be honoured alongside the subject filter")
}

// An issue and a release can share a numeric id. Because the subject filter also pins the
// source, they must never match each other.
func TestFindNotificationOptionsDoNotCollideAcrossSources(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const sharedID = int64(1)

	_, err := activities_model.CreateReleaseNotification(t.Context(), 1, 1, sharedID, receiverID, "v1.0")
	require.NoError(t, err)

	opts := activities_model.FindNotificationOptions{UserID: receiverID}
	opts.FilterByIssue(sharedID, false)
	notfs, err := db.Find[activities_model.Notification](t.Context(), opts)
	assert.NoError(t, err)
	for _, notf := range notfs {
		assert.NotEqual(t, activities_model.NotificationSourceRelease, notf.Source,
			"an issue filter must not match a release with the same id")
	}
}

func TestUpsertNotificationIsIdempotent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const repoID = int64(1)
	const releaseID = int64(1)

	// Repeated calls for the same release/user must converge to one row, covering both the
	// insert and the update-on-conflict path.
	for range 5 {
		_, err := activities_model.CreateReleaseNotification(t.Context(), 1, repoID, releaseID, receiverID, "v1.0")
		require.NoError(t, err)
	}

	opts := activities_model.FindNotificationOptions{UserID: receiverID}
	opts.FilterByRelease(releaseID)
	notfs, err := db.Find[activities_model.Notification](t.Context(), opts)
	assert.NoError(t, err)
	assert.Len(t, notfs, 1)
}

// Re-notifying an already-read notification must flip it back to unread and report the
// change, otherwise the unread badge never comes back.
func TestUpsertNotificationReopensReadNotification(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const releaseID = int64(1)

	changed, err := activities_model.CreateReleaseNotification(t.Context(), 1, 1, releaseID, receiverID, "v1.0")
	require.NoError(t, err)
	require.True(t, changed)

	_, err = activities_model.SetReleaseReadBy(t.Context(), releaseID, receiverID)
	require.NoError(t, err)

	changed, err = activities_model.CreateReleaseNotification(t.Context(), 3, 1, releaseID, receiverID, "v1.0")
	require.NoError(t, err)
	assert.True(t, changed)

	opts := activities_model.FindNotificationOptions{UserID: receiverID}
	opts.FilterByRelease(releaseID)
	notfs, err := db.Find[activities_model.Notification](t.Context(), opts)
	require.NoError(t, err)
	if assert.Len(t, notfs, 1) {
		assert.Equal(t, activities_model.NotificationStatusUnread, notfs[0].Status)
		assert.EqualValues(t, 3, notfs[0].UpdatedBy, "updated_by must move so the notification is reordered")
	}
}

func TestCreateRepoTransferNotificationDeduplicatesByRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const receiverID = int64(2)
	const repoID = int64(1)

	_, err := activities_model.CreateRepoTransferNotification(t.Context(), 1, repoID, receiverID, "user2/repo1")
	require.NoError(t, err)
	_, err = activities_model.CreateRepoTransferNotification(t.Context(), 3, repoID, receiverID, "user2/repo1")
	require.NoError(t, err)

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

// The notification page must render even when the subject is gone: a deleted release, a
// GC'd commit or a removed issue used to take the whole page down with a 500.
func TestNotificationRendersWithoutSubject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	for _, tc := range []struct {
		name string
		notf *activities_model.Notification
	}{
		{"release", &activities_model.Notification{
			Source: activities_model.NotificationSourceRelease, RepoID: repo.ID,
			SubjectID: unittest.NonexistentID, Title: "v9.9.9", Repository: repo,
		}},
		{"commit", &activities_model.Notification{
			Source: activities_model.NotificationSourceCommit, RepoID: repo.ID,
			SubjectRef: "deadbeef", Title: "a commit message", Repository: repo,
		}},
		{"issue", &activities_model.Notification{
			Source: activities_model.NotificationSourceIssue, RepoID: repo.ID,
			SubjectID: unittest.NonexistentID, Title: "a deleted issue", Repository: repo,
		}},
		{"pull request", &activities_model.Notification{
			Source: activities_model.NotificationSourcePullRequest, RepoID: repo.ID,
			SubjectID: unittest.NonexistentID, Title: "a deleted pull", Repository: repo,
		}},
		{"repository", &activities_model.Notification{
			Source: activities_model.NotificationSourceRepository, RepoID: repo.ID,
			Title: repo.FullName(), Repository: repo,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				assert.NotEmpty(t, tc.notf.DisplayTitle())
				assert.NotEmpty(t, tc.notf.Link(t.Context()))
				assert.NotEmpty(t, tc.notf.HTMLURL(t.Context()))
				assert.NotEmpty(t, tc.notf.IconHTML(t.Context()))
			})
		})
	}
}

// GetReleaseByID leaves Release.Repo nil while Release.Link() dereferences it, so a
// notification for a release that still exists used to panic on the single-load path.
func TestNotificationLoadsReleaseRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	release := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: 1})

	notf := &activities_model.Notification{
		UserID:    2,
		Source:    activities_model.NotificationSourceRelease,
		RepoID:    release.RepoID,
		SubjectID: release.ID,
	}
	require.NoError(t, notf.LoadAttributes(t.Context()))
	assert.NotPanics(t, func() {
		assert.NotEmpty(t, notf.Link(t.Context()))
		assert.NotEmpty(t, notf.HTMLURL(t.Context()))
	})
}

// A live subject wins over the snapshot, so a renamed issue shows its current title.
func TestNotificationDisplayTitlePrefersLoadedSubject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	notf := &activities_model.Notification{
		Source:    activities_model.NotificationSourceIssue,
		SubjectID: issue.ID,
		Title:     "the title when the notification was created",
		Issue:     issue,
	}
	assert.Equal(t, issue.Title, notf.DisplayTitle())

	notf.Issue = nil
	assert.Equal(t, "the title when the notification was created", notf.DisplayTitle())
}
